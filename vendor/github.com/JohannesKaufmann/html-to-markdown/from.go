// Package md converts html to markdown.
//
//  converter := md.NewConverter("", true, nil)
//
//  html = `<strong>Important</strong>`
//
//  markdown, err := converter.ConvertString(html)
//  if err != nil {
//    log.Fatal(err)
//  }
//  fmt.Println("md ->", markdown)
// Or if you are already using goquery:
//  markdown, err := converter.Convert(selec)
package md

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type simpleRuleFunc func(content string, selec *goquery.Selection, options *Options) *string
type ruleFunc func(content string, selec *goquery.Selection, options *Options) (res AdvancedResult, skip bool)

// BeforeHook runs before the converter and can be used to transform the original html
type BeforeHook func(selec *goquery.Selection)

// Afterhook runs after the converter and can be used to transform the resulting markdown
type Afterhook func(markdown string) string

// Converter is initialized by NewConverter.
type Converter struct {
	mutex  sync.RWMutex
	rules  map[string][]ruleFunc
	keep   map[string]struct{}
	remove map[string]struct{}

	before []BeforeHook
	after  []Afterhook

	domain  string
	options Options
}

func validate(val string, possible ...string) error {
	for _, e := range possible {
		if e == val {
			return nil
		}
	}
	return fmt.Errorf("field must be one of %v but got %s", possible, val)
}
func validateOptions(opt Options) error {
	if err := validate(opt.HeadingStyle, "setext", "atx"); err != nil {
		return err
	}
	if strings.Count(opt.HorizontalRule, "*") < 3 &&
		strings.Count(opt.HorizontalRule, "_") < 3 &&
		strings.Count(opt.HorizontalRule, "-") < 3 {
		return errors.New("HorizontalRule must be at least 3 characters of '*', '_' or '-' but got " + opt.HorizontalRule)
	}

	if err := validate(opt.BulletListMarker, "-", "+", "*"); err != nil {
		return err
	}
	if err := validate(opt.CodeBlockStyle, "indented", "fenced"); err != nil {
		return err
	}
	if err := validate(opt.Fence, "```", "~~~"); err != nil {
		return err
	}
	if err := validate(opt.EmDelimiter, "_", "*"); err != nil {
		return err
	}
	if err := validate(opt.StrongDelimiter, "**", "__"); err != nil {
		return err
	}
	if err := validate(opt.LinkStyle, "inlined", "referenced"); err != nil {
		return err
	}
	if err := validate(opt.LinkReferenceStyle, "full", "collapsed", "shortcut"); err != nil {
		return err
	}

	return nil
}

var (
	attrListPrefix = "data-converter-list-prefix"
)

// NewConverter initializes a new converter and holds all the rules.
// - `domain` is used for links and images to convert relative urls ("/image.png") to absolute urls.
// - CommonMark is the default set of rules. Set enableCommonmark to false if you want
//   to customize everything using AddRules and DONT want to fallback to default rules.
func NewConverter(domain string, enableCommonmark bool, options *Options) *Converter {
	conv := &Converter{
		domain: domain,
		rules:  make(map[string][]ruleFunc),
		keep:   make(map[string]struct{}),
		remove: make(map[string]struct{}),
	}

	conv.before = append(conv.before, func(selec *goquery.Selection) {
		selec.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			// TODO: don't hardcode "data-index" and rename it to avoid accidental conflicts
			s.SetAttr("data-index", strconv.Itoa(i+1))
		})
	})
	conv.before = append(conv.before, func(selec *goquery.Selection) {
		selec.Find("li").Each(func(i int, s *goquery.Selection) {
			prefix := getListPrefix(options, s)

			s.SetAttr(attrListPrefix, prefix)
		})
	})
	conv.after = append(conv.after, func(markdown string) string {
		markdown = strings.TrimSpace(markdown)
		markdown = multipleNewLinesRegex.ReplaceAllString(markdown, "\n\n")

		// remove unnecessary trailing spaces to have clean markdown
		markdown = TrimTrailingSpaces(markdown)

		return markdown
	})

	if enableCommonmark {
		conv.AddRules(commonmark...)
		conv.remove["script"] = struct{}{}
		conv.remove["style"] = struct{}{}
		conv.remove["textarea"] = struct{}{}
	}

	// TODO: put domain in options?
	if options == nil {
		options = &Options{}
	}
	if options.HeadingStyle == "" {
		options.HeadingStyle = "atx"
	}
	if options.HorizontalRule == "" {
		options.HorizontalRule = "* * *"
	}
	if options.BulletListMarker == "" {
		options.BulletListMarker = "-"
	}
	if options.CodeBlockStyle == "" {
		options.CodeBlockStyle = "indented"
	}
	if options.Fence == "" {
		options.Fence = "```"
	}
	if options.EmDelimiter == "" {
		options.EmDelimiter = "_"
	}
	if options.StrongDelimiter == "" {
		options.StrongDelimiter = "**"
	}
	if options.LinkStyle == "" {
		options.LinkStyle = "inlined"
	}
	if options.LinkReferenceStyle == "" {
		options.LinkReferenceStyle = "full"
	}
	if options.EscapeMode == "" {
		options.EscapeMode = "basic"
	}

	// for now, store it in the options
	options.domain = domain

	if options.GetAbsoluteURL == nil {
		options.GetAbsoluteURL = DefaultGetAbsoluteURL
	}

	conv.options = *options
	err := validateOptions(conv.options)
	if err != nil {
		log.Println("markdown options is not valid:", err)
	}

	return conv
}
func (conv *Converter) getRuleFuncs(tag string) []ruleFunc {
	conv.mutex.RLock()
	defer conv.mutex.RUnlock()

	r, ok := conv.rules[tag]
	if !ok || len(r) == 0 {
		if _, keep := conv.keep[tag]; keep {
			return []ruleFunc{wrap(ruleKeep)}
		}
		if _, remove := conv.remove[tag]; remove {
			return nil // TODO:
		}

		return []ruleFunc{wrap(ruleDefault)}
	}

	return r
}

func wrap(simple simpleRuleFunc) ruleFunc {
	return func(content string, selec *goquery.Selection, opt *Options) (AdvancedResult, bool) {
		res := simple(content, selec, opt)
		if res == nil {
			return AdvancedResult{}, true
		}
		return AdvancedResult{Markdown: *res}, false
	}
}

// Before registers a hook that is run before the conversion. It
// can be used to transform the original goquery html document.
//
// For example, the default before hook adds an index to every link,
// so that the `a` tag rule (for "reference" "full") can have an incremental number.
func (conv *Converter) Before(hooks ...BeforeHook) *Converter {
	conv.mutex.Lock()
	defer conv.mutex.Unlock()

	for _, hook := range hooks {
		conv.before = append(conv.before, hook)
	}

	return conv
}

// After registers a hook that is run after the conversion. It
// can be used to transform the markdown document that is about to be returned.
//
// For example, the default after hook trims the returned markdown.
func (conv *Converter) After(hooks ...Afterhook) *Converter {
	conv.mutex.Lock()
	defer conv.mutex.Unlock()

	for _, hook := range hooks {
		conv.after = append(conv.after, hook)
	}

	return conv
}

// ClearBefore clears the current before hooks (including the default before hooks).
func (conv *Converter) ClearBefore() *Converter {
	conv.mutex.Lock()
	defer conv.mutex.Unlock()

	conv.before = nil

	return conv
}

// ClearAfter clears the current after hooks (including the default after hooks).
func (conv *Converter) ClearAfter() *Converter {
	conv.mutex.Lock()
	defer conv.mutex.Unlock()

	conv.after = nil

	return conv
}

// AddRules adds the rules that are passed in to the converter.
//
// By default it overrides the rule for that html tag. You can
// fall back to the default rule by returning nil.
func (conv *Converter) AddRules(rules ...Rule) *Converter {
	conv.mutex.Lock()
	defer conv.mutex.Unlock()

	for _, rule := range rules {
		if len(rule.Filter) == 0 {
			log.Println("you need to specify at least one filter for your rule")
		}
		for _, filter := range rule.Filter {
			r, _ := conv.rules[filter]

			if rule.AdvancedReplacement != nil {
				r = append(r, rule.AdvancedReplacement)
			} else {
				r = append(r, wrap(rule.Replacement))
			}
			conv.rules[filter] = r
		}
	}

	return conv
}

// Keep certain html tags in the generated output.
func (conv *Converter) Keep(tags ...string) *Converter {
	conv.mutex.Lock()
	defer conv.mutex.Unlock()

	for _, tag := range tags {
		conv.keep[tag] = struct{}{}
	}
	return conv
}

// Remove certain html tags from the source.
func (conv *Converter) Remove(tags ...string) *Converter {
	conv.mutex.Lock()
	defer conv.mutex.Unlock()
	for _, tag := range tags {
		conv.remove[tag] = struct{}{}
	}
	return conv
}

// Plugin can be used to extends functionality beyond what
// is offered by commonmark.
type Plugin func(conv *Converter) []Rule

// Use can be used to add additional functionality to the converter. It is
// used when its not sufficient to use only rules for example in Plugins.
func (conv *Converter) Use(plugins ...Plugin) *Converter {
	for _, plugin := range plugins {
		rules := plugin(conv)
		conv.AddRules(rules...) // TODO: for better performance only use one lock for all plugins
	}
	return conv
}

// Timeout for the http client
var Timeout = time.Second * 10
var netClient = &http.Client{
	Timeout: Timeout,
}

// DomainFromURL returns `u.Host` from the parsed url.
func DomainFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)

	u, _ := url.Parse(rawURL)
	if u != nil && u.Host != "" {
		return u.Host
	}

	// lets try it again by adding a scheme
	u, _ = url.Parse("http://" + rawURL)
	if u != nil {
		return u.Host
	}

	return ""
}

// Reduce many newline characters `\n` to at most 2 new line characters.
var multipleNewLinesRegex = regexp.MustCompile(`[\n]{2,}`)

// Convert returns the content from a goquery selection.
// If you have a goquery document just pass in doc.Selection.
func (conv *Converter) Convert(selec *goquery.Selection) string {
	conv.mutex.RLock()
	domain := conv.domain
	options := conv.options
	l := len(conv.rules)
	if l == 0 {
		log.Println("you have added no rules. either enable commonmark or add you own.")
	}
	before := conv.before
	after := conv.after
	conv.mutex.RUnlock()

	// before hook
	for _, hook := range before {
		hook(selec)
	}

	res := conv.selecToMD(domain, selec, &options)
	markdown := res.Markdown

	if res.Header != "" {
		markdown = res.Header + "\n\n" + markdown
	}
	if res.Footer != "" {
		markdown += "\n\n" + res.Footer
	}

	// after hook
	for _, hook := range after {
		markdown = hook(markdown)
	}

	return markdown
}

// ConvertReader returns the content from a reader and returns a buffer.
func (conv *Converter) ConvertReader(reader io.Reader) (bytes.Buffer, error) {
	var buffer bytes.Buffer
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return buffer, err
	}
	buffer.WriteString(
		conv.Convert(doc.Selection),
	)

	return buffer, nil
}

// ConvertResponse returns the content from a html response.
func (conv *Converter) ConvertResponse(res *http.Response) (string, error) {
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		return "", err
	}
	return conv.Convert(doc.Selection), nil
}

// ConvertString returns the content from a html string. If you
// already have a goquery selection use `Convert`.
func (conv *Converter) ConvertString(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}
	return conv.Convert(doc.Selection), nil
}

// ConvertBytes returns the content from a html byte array.
func (conv *Converter) ConvertBytes(bytes []byte) ([]byte, error) {
	res, err := conv.ConvertString(string(bytes))
	if err != nil {
		return nil, err
	}
	return []byte(res), nil
}

// ConvertURL returns the content from the page with that url.
func (conv *Converter) ConvertURL(url string) (string, error) {
	// not using goquery.NewDocument directly because of the timeout
	resp, err := netClient.Get(url)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("expected a status code in the 2xx range but got %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return "", err
	}
	domain := DomainFromURL(url)
	if conv.domain != domain {
		log.Printf("expected '%s' as the domain but got '%s' \n", conv.domain, domain)
	}
	return conv.Convert(doc.Selection), nil
}
