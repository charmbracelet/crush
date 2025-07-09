// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genai

import (
	"context"
	"errors"
	"iter"
)

// ErrPageDone is the error returned by an iterator's Next method when no more pages are available.
var ErrPageDone = errors.New("no more pages")

// Page represents a page of results from a paginated API call.
// It contains a slice of items and information about the next page.
type Page[T any] struct {
	Name          string // The name of the resource.
	Items         []*T   // The items in the current page.
	NextPageToken string // The token to use to retrieve the next page of results.

	config   map[string]any                                                         // The configuration used for the API call.
	listFunc func(ctx context.Context, config map[string]any) ([]*T, string, error) // The function used to retrieve the next page.
}

func newPage[T any](ctx context.Context, name string, config map[string]any, listFunc func(ctx context.Context, config map[string]any) ([]*T, string, error)) (Page[T], error) {
	p := Page[T]{
		Name:     name,
		config:   config,
		listFunc: listFunc,
	}
	items, nextPageToken, err := listFunc(ctx, config)
	if err != nil {
		return p, err
	}
	p.Items = items
	p.NextPageToken = nextPageToken
	return p, nil
}

// all returns an iterator that yields all items across all pages of results.
//
// The iterator retrieves each page sequentially and yields each item within
// the page.  If an error occurs during retrieval, the iterator will stop
// and the error will be returned as the second value in the next call to Next().
// A genai.PageDone error indicates that all pages have been processed.
func (p Page[T]) all(ctx context.Context) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		for {
			for _, item := range p.Items {
				if !yield(item, nil) {
					return
				}
			}
			var err error
			p, err = p.Next(ctx)
			if err == ErrPageDone {
				return
			}
			if err != nil {
				yield(nil, err)
				return
			}
		}
	}
}

// Next retrieves the next page of results.
//
// If there are no more pages, PageDone is returned.  Otherwise,
// a new Page struct containing the next set of results is returned.
// Any other errors encountered during retrieval will also be returned.
func (p Page[T]) Next(ctx context.Context) (Page[T], error) {
	if p.NextPageToken == "" {
		return p, ErrPageDone
	}
	c := make(map[string]any)
	for k, v := range p.config {
		c[k] = v
	}
	c["PageToken"] = p.NextPageToken

	return newPage[T](ctx, p.Name, c, p.listFunc)
}
