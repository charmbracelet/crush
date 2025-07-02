# Changelog

## 1.4.0 (2025-06-04)

Full Changelog: [v1.3.0...v1.4.0](https://github.com/anthropics/anthropic-sdk-go/compare/v1.3.0...v1.4.0)

### Features

* **client:** allow overriding unions ([079149c](https://github.com/anthropics/anthropic-sdk-go/commit/079149c673981891ecd35906cd610f8d4a4b69a9))


### Chores

* **internal:** codegen related update ([853ba1f](https://github.com/anthropics/anthropic-sdk-go/commit/853ba1f46d2b6c476ee04d9c061368e708cc9e18))

## 1.3.0 (2025-06-03)

Full Changelog: [v1.2.2...v1.3.0](https://github.com/anthropics/anthropic-sdk-go/compare/v1.2.2...v1.3.0)

### Features

* **client:** add support for new text_editor_20250429 tool ([b33c543](https://github.com/anthropics/anthropic-sdk-go/commit/b33c543f7dc3b74c3322b6f84c189b81f67b6154))

## 1.2.2 (2025-06-02)

Full Changelog: [v1.2.1...v1.2.2](https://github.com/anthropics/anthropic-sdk-go/compare/v1.2.1...v1.2.2)

### Bug Fixes

* **client:** access subunions properly ([f29c162](https://github.com/anthropics/anthropic-sdk-go/commit/f29c1627fe94c6371937659d02f1af7b55583d60))
* fix error ([bbc002c](https://github.com/anthropics/anthropic-sdk-go/commit/bbc002ccbbf9df681201d9b8ba806c37338c0fd3))


### Chores

* make go mod tidy continue on error ([ac184b4](https://github.com/anthropics/anthropic-sdk-go/commit/ac184b4f7afee4015d133a05ce819a8dac35be52))

## 1.2.1 (2025-05-23)

Full Changelog: [v1.2.0...v1.2.1](https://github.com/anthropics/anthropic-sdk-go/compare/v1.2.0...v1.2.1)

### Chores

* **examples:** clean up MCP example ([66f406a](https://github.com/anthropics/anthropic-sdk-go/commit/66f406a04b9756281e7716e9b635c3e3f29397fb))
* **internal:** fix release workflows ([6a0ff4c](https://github.com/anthropics/anthropic-sdk-go/commit/6a0ff4cad1c1b4ab6435df80fccd945d6ce07be7))

## 1.2.0 (2025-05-22)

Full Changelog: [v1.1.0...v1.2.0](https://github.com/anthropics/anthropic-sdk-go/compare/v1.1.0...v1.2.0)

### Features

* **api:** add claude 4 models, files API, code execution tool, MCP connector and more ([b2e5cbf](https://github.com/anthropics/anthropic-sdk-go/commit/b2e5cbffd9d05228c2c2569974a6fa260c3f46be))


### Bug Fixes

* **tests:** fix model testing for anthropic.CalculateNonStreamingTimeout ([9956842](https://github.com/anthropics/anthropic-sdk-go/commit/995684240b77284a4590b1b9ae34a85e525d1e52))

## 1.1.0 (2025-05-22)

Full Changelog: [v1.0.0...v1.1.0](https://github.com/anthropics/anthropic-sdk-go/compare/v1.0.0...v1.1.0)

### Features

* **api:** add claude 4 models, files API, code execution tool, MCP connector and more ([2740935](https://github.com/anthropics/anthropic-sdk-go/commit/2740935f444de2d46103a7c777ea75e7e214872e))


### Bug Fixes

* **tests:** fix model testing for anthropic.CalculateNonStreamingTimeout ([f1aa0a1](https://github.com/anthropics/anthropic-sdk-go/commit/f1aa0a1a32d1ca87b87a7d688daab31f2a36071c))

## 1.0.0 (2025-05-21)

Full Changelog: [v0.2.0-beta.4...v1.0.0](https://github.com/anthropics/anthropic-sdk-go/compare/v0.2.0-beta.4...v1.0.0)

### ⚠ BREAKING CHANGES

* **client:** rename variant constructors
* **client:** remove is present

### Features

* **client:** improve variant constructor names ([227c96b](https://github.com/anthropics/anthropic-sdk-go/commit/227c96bf50e14827e112c31ad0f512354477a409))
* **client:** rename variant constructors ([078fad6](https://github.com/anthropics/anthropic-sdk-go/commit/078fad6558642a20b5fb3e82186b03c2efc0ab47))


### Bug Fixes

* **client:** correctly set stream key for multipart ([f17bfe0](https://github.com/anthropics/anthropic-sdk-go/commit/f17bfe0aac0fb8228d9cad87ccca0deb7449a824))
* **client:** don't panic on marshal with extra null field ([d67a151](https://github.com/anthropics/anthropic-sdk-go/commit/d67a151a6ef0870918c5eaf84ce996cb5b1860b7))
* **client:** elide nil citations array ([09cadec](https://github.com/anthropics/anthropic-sdk-go/commit/09cadec3c076d74bda74e67c345a1aee1fdb7ce4))
* **client:** fix bug with empty tool inputs and citation deltas in Accumulate ([f4ac348](https://github.com/anthropics/anthropic-sdk-go/commit/f4ac348658fb83485d6555c63f90920599c98d99))
* **client:** increase max stream buffer size ([18a6ccf](https://github.com/anthropics/anthropic-sdk-go/commit/18a6ccf1961922a342467800c737fa000bdd254e))
* **client:** remove is present ([385d99f](https://github.com/anthropics/anthropic-sdk-go/commit/385d99fa225c755d9af737425ad2ef4d66ad5ba9))
* **client:** resolve naming collisions in union variants ([2cb6904](https://github.com/anthropics/anthropic-sdk-go/commit/2cb69048a6b583954934bc2926186564b5c74bf6))
* **client:** use scanner for streaming ([82a2840](https://github.com/anthropics/anthropic-sdk-go/commit/82a2840ce0f8aa8bd63f7697c566f437c06bb132))


### Chores

* **examples:** remove fmt ([872e055](https://github.com/anthropics/anthropic-sdk-go/commit/872e0550171942c405786c7eedb23b8270f6e8de))
* formatting ([1ce0ee8](https://github.com/anthropics/anthropic-sdk-go/commit/1ce0ee863c5df658909d81b138dc1ebedb78844a))
* improve devcontainer setup ([9021490](https://github.com/anthropics/anthropic-sdk-go/commit/90214901d77ba57901e77d6ea31aafb06c120f2c))


### Documentation

* upgrade security note to warning ([#346](https://github.com/anthropics/anthropic-sdk-go/issues/346)) ([83e70de](https://github.com/anthropics/anthropic-sdk-go/commit/83e70decfb5da14a1ecf78402302f7f0600515ea))

## 0.2.0-beta.4 (2025-05-18)

Full Changelog: [v0.2.0-beta.3...v0.2.0-beta.4](https://github.com/anthropics/anthropic-sdk-go/compare/v0.2.0-beta.3...v0.2.0-beta.4)

### ⚠ BREAKING CHANGES

* **client:** clearer array variant names
* **client:** rename resp package
* **client:** improve core function names
* **client:** improve union variant names
* **client:** improve param subunions & deduplicate types

### Features

* **api:** adds web search capabilities to the Claude API ([9ca314a](https://github.com/anthropics/anthropic-sdk-go/commit/9ca314a74998f24b5f17427698a8fa709b103581))
* **api:** extract ContentBlockDelta events into their own schemas ([#165](https://github.com/anthropics/anthropic-sdk-go/issues/165)) ([6d75486](https://github.com/anthropics/anthropic-sdk-go/commit/6d75486e9f524f5511f787181106a679e3414498))
* **api:** manual updates ([d405f97](https://github.com/anthropics/anthropic-sdk-go/commit/d405f97373cd7ae863a7400441d1d79c85f0ddd5))
* **api:** manual updates ([e1326cd](https://github.com/anthropics/anthropic-sdk-go/commit/e1326cdd756beb871e939af8be8b45fd3d5fdc9a))
* **api:** manual updates ([a92a382](https://github.com/anthropics/anthropic-sdk-go/commit/a92a382976d595dd32208109b480bf26dbbdc00f))
* **api:** manual updates ([59bd507](https://github.com/anthropics/anthropic-sdk-go/commit/59bd5071282403373ddca9333fafc9efc90a16d6))
* **client:** add dynamic streaming buffer to handle large lines ([510e099](https://github.com/anthropics/anthropic-sdk-go/commit/510e099e19fa71411502650eb387f1fee79f5d0d))
* **client:** add escape hatch to omit required param fields ([#175](https://github.com/anthropics/anthropic-sdk-go/issues/175)) ([6df8184](https://github.com/anthropics/anthropic-sdk-go/commit/6df8184947d6568260fa0bc22a89a27d10eaacd0))
* **client:** add helper method to generate constant structs ([015e8bc](https://github.com/anthropics/anthropic-sdk-go/commit/015e8bc7f74582fb5a3d69021ad3d61e96d65b36))
* **client:** add support for endpoint-specific base URLs in python ([44645c9](https://github.com/anthropics/anthropic-sdk-go/commit/44645c9fd0b883db4deeb88bfee6922ec9845ace))
* **client:** add support for reading base URL from environment variable ([835e632](https://github.com/anthropics/anthropic-sdk-go/commit/835e6326b658cd40590cd8bbed0932ab219e6d2d))
* **client:** clearer array variant names ([1fdea8f](https://github.com/anthropics/anthropic-sdk-go/commit/1fdea8f9fedc470a917d12607b3b7ebe3f0f6439))
* **client:** experimental support for unmarshalling into param structs ([94c8fa4](https://github.com/anthropics/anthropic-sdk-go/commit/94c8fa41ecb4792cb7da043bde2c0f5ddafe84b0))
* **client:** improve param subunions & deduplicate types ([8daacf6](https://github.com/anthropics/anthropic-sdk-go/commit/8daacf6866e8bc706ec29e17046e53d4ed100364))
* **client:** make response union's AsAny method type safe ([#174](https://github.com/anthropics/anthropic-sdk-go/issues/174)) ([f410ed0](https://github.com/anthropics/anthropic-sdk-go/commit/f410ed025ee57a05b0cec8d72a1cb43d30e564a6))
* **client:** rename resp package ([8e7d278](https://github.com/anthropics/anthropic-sdk-go/commit/8e7d2788e9be7b954d07de731e7b27ad2e2a9e8e))
* **client:** support custom http clients ([#177](https://github.com/anthropics/anthropic-sdk-go/issues/177)) ([ff7a793](https://github.com/anthropics/anthropic-sdk-go/commit/ff7a793b43b99dc148b30e408edfdc19e19c28b2))
* **client:** support more time formats ([af2df86](https://github.com/anthropics/anthropic-sdk-go/commit/af2df86f24acbe6b9cdcc4e055c3ff754303e0ef))
* **client:** support param struct overrides ([#167](https://github.com/anthropics/anthropic-sdk-go/issues/167)) ([e0d5eb0](https://github.com/anthropics/anthropic-sdk-go/commit/e0d5eb098c6441e99d53c6d997c7bcca460a238b))
* **client:** support unions in query and forms ([#171](https://github.com/anthropics/anthropic-sdk-go/issues/171)) ([6bf1ce3](https://github.com/anthropics/anthropic-sdk-go/commit/6bf1ce36f0155dba20afd4b63bf96c4527e2baa5))


### Bug Fixes

* **client:** clean up reader resources ([2234386](https://github.com/anthropics/anthropic-sdk-go/commit/223438673ade3be3435bebf7063fd34ddf3dfb8e))
* **client:** correctly update body in WithJSONSet ([f531c77](https://github.com/anthropics/anthropic-sdk-go/commit/f531c77c15859b1f2e61d654f4d9956cdfafa082))
* **client:** deduplicate stop reason type ([#155](https://github.com/anthropics/anthropic-sdk-go/issues/155)) ([0f985ad](https://github.com/anthropics/anthropic-sdk-go/commit/0f985ad54ef47849d7d478c84d34c7350a4349b5))
* **client:** fix bug where types occasionally wouldn't generate ([8988713](https://github.com/anthropics/anthropic-sdk-go/commit/8988713904ce73d3c82de635d98da48b98532366))
* **client:** improve core function names ([0a2777f](https://github.com/anthropics/anthropic-sdk-go/commit/0a2777fd597a5eb74bcf6b1da48a9ff1988059de))
* **client:** improve union variant names ([92718fd](https://github.com/anthropics/anthropic-sdk-go/commit/92718fd4058fd8535fd888a56f83fc2d3ec505ef))
* **client:** include path for type names in example code ([5bbe836](https://github.com/anthropics/anthropic-sdk-go/commit/5bbe83639793878aa0ea52e8ff06b1d9ee72ed7c))
* **client:** resolve issue with optional multipart files ([e2af94c](https://github.com/anthropics/anthropic-sdk-go/commit/e2af94c840a8f9da566c781fc99c57084e490ec1))
* **client:** return error on bad custom url instead of panic ([#169](https://github.com/anthropics/anthropic-sdk-go/issues/169)) ([b086b55](https://github.com/anthropics/anthropic-sdk-go/commit/b086b55f4886474282d4e2ea9ee3495cbf25ec6b))
* **client:** support multipart encoding array formats ([#170](https://github.com/anthropics/anthropic-sdk-go/issues/170)) ([611a25a](https://github.com/anthropics/anthropic-sdk-go/commit/611a25a427fc5303bb311fa4a2fec836d55b0933))
* **client:** time format encoding fix ([d589846](https://github.com/anthropics/anthropic-sdk-go/commit/d589846c1a08ad56d639d60736e2b8e190f7f2b1))
* **client:** unmarshal responses properly ([8344a1c](https://github.com/anthropics/anthropic-sdk-go/commit/8344a1c58dd497abbed8e9e689efca544256eaa8))
* **client:** unmarshal stream events into fresh memory ([#168](https://github.com/anthropics/anthropic-sdk-go/issues/168)) ([9cc1257](https://github.com/anthropics/anthropic-sdk-go/commit/9cc1257a67340e446ac415ec9ddddded24bb1f9a))
* handle empty bodies in WithJSONSet ([0bad01e](https://github.com/anthropics/anthropic-sdk-go/commit/0bad01e40a2a4b5b376ba27513d7e16d604459d9))
* **internal:** fix type changes ([d8ef353](https://github.com/anthropics/anthropic-sdk-go/commit/d8ef3531840ac1dc0541d3b1cf0015d1db29e2b6))
* **pagination:** handle errors when applying options ([2381476](https://github.com/anthropics/anthropic-sdk-go/commit/2381476e64991e781b696890c98f78001e256b3b))


### Chores

* **ci:** add timeout thresholds for CI jobs ([335e9f0](https://github.com/anthropics/anthropic-sdk-go/commit/335e9f0af2275f1af21aa7062afb50bee81771b6))
* **ci:** only use depot for staging repos ([6818451](https://github.com/anthropics/anthropic-sdk-go/commit/68184515143aa1e4473208f794fa593668c94df4))
* **ci:** run on more branches and use depot runners ([b0ca09d](https://github.com/anthropics/anthropic-sdk-go/commit/b0ca09d1d39a8de390c47be804847a7647ca3c67))
* **client:** use new opt conversion ([#184](https://github.com/anthropics/anthropic-sdk-go/issues/184)) ([58dc74f](https://github.com/anthropics/anthropic-sdk-go/commit/58dc74f951aa6a0eb4355a0213c8695bfa7cb0ed))
* **docs:** doc improvements ([#173](https://github.com/anthropics/anthropic-sdk-go/issues/173)) ([aebe8f6](https://github.com/anthropics/anthropic-sdk-go/commit/aebe8f68afa3de4460cda6e4032c7859e13cda81))
* **docs:** document pre-request options ([8f5eb18](https://github.com/anthropics/anthropic-sdk-go/commit/8f5eb188146bd46ba990558a7e2348c8697d6405))
* **docs:** readme improvements ([#176](https://github.com/anthropics/anthropic-sdk-go/issues/176)) ([b5769ff](https://github.com/anthropics/anthropic-sdk-go/commit/b5769ffcf5ef5345659ae848b875227718ea2425))
* **docs:** update file uploads in README ([#166](https://github.com/anthropics/anthropic-sdk-go/issues/166)) ([a4a36bf](https://github.com/anthropics/anthropic-sdk-go/commit/a4a36bfbefa5a166774c23d8c5428fb55c1b4abe))
* **docs:** update respjson package name ([28910b5](https://github.com/anthropics/anthropic-sdk-go/commit/28910b57821cab670561a25bee413375187ed747))
* **internal:** expand CI branch coverage ([#178](https://github.com/anthropics/anthropic-sdk-go/issues/178)) ([900e2df](https://github.com/anthropics/anthropic-sdk-go/commit/900e2df3eb2d3e1309d85fdcf807998f701bea8a))
* **internal:** reduce CI branch coverage ([343f6c6](https://github.com/anthropics/anthropic-sdk-go/commit/343f6c6c295dc3d39f65aae481bc10969dbb5694))
* **internal:** remove CI condition ([#160](https://github.com/anthropics/anthropic-sdk-go/issues/160)) ([adfa1e2](https://github.com/anthropics/anthropic-sdk-go/commit/adfa1e2e349842aa88262af70b209d1a59dbb419))
* **internal:** update config ([#157](https://github.com/anthropics/anthropic-sdk-go/issues/157)) ([46f0194](https://github.com/anthropics/anthropic-sdk-go/commit/46f019497bd9533390c4b9f0ebee6863263ce009))
* **readme:** improve formatting ([66be9bb](https://github.com/anthropics/anthropic-sdk-go/commit/66be9bbb17ccc9d878e79b3c39605da3e2846297))


### Documentation

* remove or fix invalid readme examples ([142576c](https://github.com/anthropics/anthropic-sdk-go/commit/142576c73b4dab5b84a2bf2481506ad642ad31cc))
* update documentation links to be more uniform ([457122b](https://github.com/anthropics/anthropic-sdk-go/commit/457122b79646dc17fa8752c98dbf4991edffc548))

## 0.2.0-beta.3 (2025-03-27)

Full Changelog: [v0.2.0-beta.2...v0.2.0-beta.3](https://github.com/anthropics/anthropic-sdk-go/compare/v0.2.0-beta.2...v0.2.0-beta.3)

### Chores

* add hash of OpenAPI spec/config inputs to .stats.yml ([#154](https://github.com/anthropics/anthropic-sdk-go/issues/154)) ([76b91b5](https://github.com/anthropics/anthropic-sdk-go/commit/76b91b56fbf42fe8982e7b861885db179b1bdcc5))
* fix typos ([#152](https://github.com/anthropics/anthropic-sdk-go/issues/152)) ([1cf6a6a](https://github.com/anthropics/anthropic-sdk-go/commit/1cf6a6ae25231b88d2eedbe0758f1281cbe439d8))

## 0.2.0-beta.2 (2025-03-25)

Full Changelog: [v0.2.0-beta.1...v0.2.0-beta.2](https://github.com/anthropics/anthropic-sdk-go/compare/v0.2.0-beta.1...v0.2.0-beta.2)

### Bug Fixes

* **client:** use raw json for tool input ([1013c2b](https://github.com/anthropics/anthropic-sdk-go/commit/1013c2bdb87a27d2420dbe0dcadc57d1fe3589f2))


### Chores

* add request options to client tests ([#150](https://github.com/anthropics/anthropic-sdk-go/issues/150)) ([7c70ae1](https://github.com/anthropics/anthropic-sdk-go/commit/7c70ae134a345aff775694abcad255c76e7dfcba))

## 0.2.0-beta.1 (2025-03-25)

Full Changelog: [v0.2.0-alpha.13...v0.2.0-beta.1](https://github.com/anthropics/anthropic-sdk-go/compare/v0.2.0-alpha.13...v0.2.0-beta.1)

### ⚠ BREAKING CHANGES

* **api:** migrate to v2

### Features

* add SKIP_BREW env var to ./scripts/bootstrap ([#137](https://github.com/anthropics/anthropic-sdk-go/issues/137)) ([4057111](https://github.com/anthropics/anthropic-sdk-go/commit/40571110129d5c66f171ead36f5d725663262bc4))
* **api:** migrate to v2 ([fcd95eb](https://github.com/anthropics/anthropic-sdk-go/commit/fcd95eb8f45d0ffedcd1e47cd0879d7e66783540))
* **client:** accept RFC6838 JSON content types ([#139](https://github.com/anthropics/anthropic-sdk-go/issues/139)) ([78d17cd](https://github.com/anthropics/anthropic-sdk-go/commit/78d17cd4122893ba62b1e14714a1da004c128344))
* **client:** allow custom baseurls without trailing slash ([#135](https://github.com/anthropics/anthropic-sdk-go/issues/135)) ([9b30fce](https://github.com/anthropics/anthropic-sdk-go/commit/9b30fce0a71a35910315e02cd3a2f2afc1fd7962))
* **client:** improve default client options support ([07f82a6](https://github.com/anthropics/anthropic-sdk-go/commit/07f82a6f9e07bf9aadf4ca150287887cb9e75bc4))
* **client:** improve default client options support ([#142](https://github.com/anthropics/anthropic-sdk-go/issues/142)) ([f261355](https://github.com/anthropics/anthropic-sdk-go/commit/f261355e497748bcb112eecb67a95d7c7c5075c0))
* **client:** support v2 ([#147](https://github.com/anthropics/anthropic-sdk-go/issues/147)) ([6b3af98](https://github.com/anthropics/anthropic-sdk-go/commit/6b3af98e02a9b6126bd715d43f83b8adf8b861e8))


### Chores

* **docs:** clarify breaking changes ([#146](https://github.com/anthropics/anthropic-sdk-go/issues/146)) ([a2586b4](https://github.com/anthropics/anthropic-sdk-go/commit/a2586b4beb2b9a0ad252e90223fbb471e6c25bc1))
* **internal:** codegen metadata ([ce0eca2](https://github.com/anthropics/anthropic-sdk-go/commit/ce0eca25c6a83fca9ececccb41faf04e74566e2d))
* **internal:** remove extra empty newlines ([#143](https://github.com/anthropics/anthropic-sdk-go/issues/143)) ([2ed1584](https://github.com/anthropics/anthropic-sdk-go/commit/2ed1584c7d80fddf2ef5143eabbd33b8f1a4603d))


### Refactors

* tidy up dependencies ([#140](https://github.com/anthropics/anthropic-sdk-go/issues/140)) ([289cc1b](https://github.com/anthropics/anthropic-sdk-go/commit/289cc1b007094421305dfc4ef01ae68bb2d50ee5))
