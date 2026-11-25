# Provider plugins

A provider plugin teaches Crush about a model provider it doesn't ship with.
It's a Bash script that uses the same `provider add` and `model add` commands
you'd put in a `crushrc`. If you've written a `crushrc` before, you already
know most of this.

This page walks through the common cases. For every flag in detail, see the
[`provider` reference](../config/README.md#provider).

## The simplest one

Most providers just want a URL and a key:

```bash
# ~/.config/crush/plugins/example.sh
provider add example \
  --type completions \
  --base-url "https://api.example.com/v1" \
  --api-key "$EXAMPLE_API_KEY"

model add example/code-1 --name "Code 1" --context-window 200000
```

Save that, export `EXAMPLE_API_KEY`, and start Crush. "Code 1" shows up in the
model picker.

`--type` is the API the provider speaks: `completions` for OpenAI-style chat
completions, `responses` for the OpenAI Responses API, and `messages` for
Anthropic-style APIs.

If the provider has a `/models` endpoint, add `--discover-models true` and
you can skip the `model add` lines.

## Signing in with OAuth

Some providers want you to sign in instead of pasting a key. Tell Crush where
the authorization server is and which client to use:

```bash
provider add example \
  --type completions \
  --base-url "https://api.example.com/v1" \
  --oauth-issuer "https://auth.example.com" \
  --oauth-client-id crush \
  --oauth-scope openid \
  --oauth-scope offline_access
```

Then run `crush login example`, or just pick one of its models in the model
dialog and Crush will ask you to sign in. Until you do, the provider shows up
in the picker but isn't treated as set up.

A few things happen for you:

- Crush finds the authorization and token endpoints from the issuer's
  well-known documents, so usually the issuer is all you need.
- It opens your browser and catches the redirect on a local port. When it
  can't do that (say, over SSH), it falls back to the device flow, where you
  type a code on another device. Use `--oauth-flow browser` or
  `--oauth-flow device` to force one.
- Tokens are saved and refreshed on their own, same as the built-in
  subscription providers.
- Once you're signed in, Crush fetches the models your account can use.

Not every server is that tidy. If yours doesn't publish its endpoints, or
only accepts one fixed redirect URL, spell them out with `--oauth-auth-url`,
`--oauth-token-url`, `--oauth-device-url`, and `--oauth-redirect-uri`. Extra
parameters like `audience` go in `--oauth-param`. Some token endpoints
want extra headers or a JSON body; `--oauth-token-header` and
`--oauth-token-encoding json` cover that.

Keep client secrets out of the script itself. Read them from an environment
variable instead:

```bash
provider add example \
  --oauth-client-id "$EXAMPLE_CLIENT_ID" \
  --oauth-client-secret "$EXAMPLE_CLIENT_SECRET"
```

## Showing how much quota is left

Subscription plans usually have limits, like "this much every five hours".
If the provider has an endpoint that reports what's left, point Crush at it:

```bash
provider add example \
  --usage-url "https://api.example.com/v1/remaining-quota" \
  --usage-groups groups
```

Now `crush usage` prints it:

```text
$ crush usage
Example Subscription
  Models · Weekly Limit Remaining: 99% left · resets in 6d 22h
  Models · Five Hour Limit Remaining: 99% left · resets in 3h 57m
```

The same numbers show up in the header and sidebar while you work.

Every response looks different, so the `--usage-*` flags tell Crush which
fields to read. They default to a common shape (groups of `buckets`, each
with a `displayName`, `remainingFraction`, and `resetTime`). If your
provider's JSON matches, you're done. If not, the
[reference](../config/README.md#remaining-quota) explains each flag.

If the plan has separate limits per model family, map model IDs to groups so
Crush only shows the limits that apply to the model you're using:

```bash
provider add example \
  --usage-model-group fast "Fast models" \
  --usage-model-group pro "Pro models"
```

## Reading the provider's model list

`--discover-models` only gets model IDs. Some providers list a lot more:
display names, context windows, which reasoning levels each model supports.
To use all of that, give Crush the URL and a small
[jq](https://jqlang.org) program that turns the response into models:

```bash
provider add example --type messages --base-url "https://api.example.com" \
  --catalog-url "/v1/models" \
  --catalog-program '
    .data | map({
      id,
      name: (.display_name // .id),
      context_window: (.max_input_tokens // 0)
    })'
```

A URL that starts with `/` is added onto `--base-url`. Your login token or API
key is sent along automatically.

Anything you declare with `model add` still wins over what the list says, so
you can pin one setting on a model without losing the rest:

```bash
model add example/big-model --reasoning-effort medium
```

If the request fails, Crush keeps going with whatever models you declared.

## When the API doesn't quite match

Sometimes a provider is *almost* a standard API but wraps requests in its own
envelope, or names things differently. Instead of waiting for Crush to
support it, you can rewrite the traffic with jq using `--gateway-request` and
`--gateway-response`. This is an advanced feature; the
[gateway adapter reference](../config/README.md#gateway-adapters) has the
details and an example.

## Pricing

Crush has no way to ask a provider what it charges, so prices come from your
config. For a subscription, set `--flat-rate true` and Crush stops counting
per-token cost, which is accurate for a plan you pay for monthly. If you
still want numbers, add them with `model add --price-input` and friends.
Since a plugin is just Bash, you can even fetch them:

```bash
price=$(curl -fsSL "https://example.com/prices.json" | jq -r '.["code-1"].input')
model add example/code-1 --price-input "$price"
```

## Tips

- Plugins run every time Crush starts, so keep them fast. Avoid slow network
  calls if you can.
- Errors stop Crush from starting and tell you which plugin failed. Run
  `crush` from a terminal to see the message.
- You can override anything a plugin sets from your own `crushrc`. To turn a
  provider off without deleting the plugin: `provider add example --disable true`.
- Publishing your plugin? Put the `.sh` files at the root of a GitHub repo
  and people can install it with `crush plugin install you/your-repo`.
