# Changelog

## 1.8.2 (2025-06-27)

Full Changelog: [v1.8.1...v1.8.2](https://github.com/openai/openai-go/compare/v1.8.1...v1.8.2)

### Bug Fixes

* don't try to deserialize as json when ResponseBodyInto is []byte ([74ad0f8](https://github.com/openai/openai-go/commit/74ad0f8fab0f956234503a9ba26fbd395944dcf8))
* **pagination:** check if page data is empty in GetNextPage ([c9becdc](https://github.com/openai/openai-go/commit/c9becdc9908f2a1961160837c6ab8cd9064e7854))

## 1.8.1 (2025-06-26)

Full Changelog: [v1.8.0...v1.8.1](https://github.com/openai/openai-go/compare/v1.8.0...v1.8.1)

### Chores

* **api:** remove unsupported property ([e22316a](https://github.com/openai/openai-go/commit/e22316adcd8f2c5aa672b12453cbd287de0e1878))
* **docs:** update README to include links to docs on Webhooks ([7bb8f85](https://github.com/openai/openai-go/commit/7bb8f8549fdd98997b1d145cbae98ff0146b4e43))

## 1.8.0 (2025-06-26)

Full Changelog: [v1.7.0...v1.8.0](https://github.com/openai/openai-go/compare/v1.7.0...v1.8.0)

### Features

* **api:** webhook and deep research support ([f6a7e7d](https://github.com/openai/openai-go/commit/f6a7e7dcd8801facc4f8d981f1ca43786c10de1e))


### Chores

* **internal:** add tests for breaking change detection ([339522d](https://github.com/openai/openai-go/commit/339522d38cd31b0753a8df37b8924f7e7dfb0b1d))

## 1.7.0 (2025-06-23)

Full Changelog: [v1.6.0...v1.7.0](https://github.com/openai/openai-go/compare/v1.6.0...v1.7.0)

### Features

* **api:** make model and inputs not required to create response ([19f0b76](https://github.com/openai/openai-go/commit/19f0b76378d35b3d81c60c85bf2e64d6bf85b9c2))
* **api:** update api shapes for usage and code interpreter ([d24d42c](https://github.com/openai/openai-go/commit/d24d42cba60e565627e8ffb1cac63a5085ddb6da))
* **client:** add escape hatch for null slice & maps ([9c633d6](https://github.com/openai/openai-go/commit/9c633d6f1dbcc0b153f42f831ee7e13d6fe62296))


### Chores

* fix documentation of null map ([8f3a134](https://github.com/openai/openai-go/commit/8f3a134e500b1b7791ab855adaef2d7b10d2d1c3))

## 1.6.0 (2025-06-17)

Full Changelog: [v1.5.0...v1.6.0](https://github.com/openai/openai-go/compare/v1.5.0...v1.6.0)

### Features

* **api:** add reusable prompt IDs ([280c698](https://github.com/openai/openai-go/commit/280c698015eba5f6bd47e2fce038eb401f6ef0f2))
* **api:** manual updates ([740f840](https://github.com/openai/openai-go/commit/740f84006ac283a25f5ad96aaf845a3c8a51c6ac))
* **client:** add debug log helper ([5715c49](https://github.com/openai/openai-go/commit/5715c491c483f8dab4ea2a900c400384f6810024))


### Chores

* **ci:** enable for pull requests ([9ed793a](https://github.com/openai/openai-go/commit/9ed793a51010423db464a7b7bd263d2fd275967f))

## 1.5.0 (2025-06-10)

Full Changelog: [v1.4.0...v1.5.0](https://github.com/openai/openai-go/compare/v1.4.0...v1.5.0)

### Features

* **api:** Add o3-pro model IDs ([3bbd0b8](https://github.com/openai/openai-go/commit/3bbd0b8f09030a6c571900d444742c4fc2a3c211))

## 1.4.0 (2025-06-09)

Full Changelog: [v1.3.0...v1.4.0](https://github.com/openai/openai-go/compare/v1.3.0...v1.4.0)

### Features

* **client:** allow overriding unions ([27c6299](https://github.com/openai/openai-go/commit/27c6299cb4ac275c6542b5691d81b795e65eeff6))


### Bug Fixes

* **client:** cast to raw message when converting to params ([a3282b0](https://github.com/openai/openai-go/commit/a3282b01a8d9a2c0cd04f24b298bf2ffcd160ebd))

## 1.3.0 (2025-06-03)

Full Changelog: [v1.2.1...v1.3.0](https://github.com/openai/openai-go/compare/v1.2.1...v1.3.0)

### Features

* **api:** add new realtime and audio models, realtime session options ([8b8f62b](https://github.com/openai/openai-go/commit/8b8f62b8e185f3fe4aaa99e892df5d35638931a1))

## 1.2.1 (2025-06-02)

Full Changelog: [v1.2.0...v1.2.1](https://github.com/openai/openai-go/compare/v1.2.0...v1.2.1)

### Bug Fixes

* **api:** Fix evals and code interpreter interfaces ([7e244c7](https://github.com/openai/openai-go/commit/7e244c73caad6b4768cced9a798452f03b1165c8))
* fix error ([a200fca](https://github.com/openai/openai-go/commit/a200fca92c3fa413cf724f424077d1537fa2ca3e))


### Chores

* make go mod tidy continue on error ([48f41c2](https://github.com/openai/openai-go/commit/48f41c2993bf6181018da859ae759951261f9ee2))

## 1.2.0 (2025-05-29)

Full Changelog: [v1.1.0...v1.2.0](https://github.com/openai/openai-go/compare/v1.1.0...v1.2.0)

### Features

* **api:** Config update for pakrym-stream-param ([84d59d5](https://github.com/openai/openai-go/commit/84d59d5cbc7521ddcc04435317903fd4ec3d17f6))


### Bug Fixes

* **client:** return binary content from `get /containers/{container_id}/files/{file_id}/content` ([f8c8de1](https://github.com/openai/openai-go/commit/f8c8de18b720b224267d54da53d7d919ed0fdff3))


### Chores

* deprecate Assistants API ([027470e](https://github.com/openai/openai-go/commit/027470e066ea6bbca1aeeb4fb9a8a3430babb84c))
* **internal:** fix release workflows ([fd46533](https://github.com/openai/openai-go/commit/fd4653316312755ccab7435fca9fb0a2d8bf8fbb))

## 1.1.0 (2025-05-22)

Full Changelog: [v1.0.0...v1.1.0](https://github.com/openai/openai-go/compare/v1.0.0...v1.1.0)

### Features

* **api:** add container endpoint ([2bd777d](https://github.com/openai/openai-go/commit/2bd777d6813b5dfcd3a2d339047a944c478dcd64))
* **api:** new API tools ([e7e2123](https://github.com/openai/openai-go/commit/e7e2123de7cafef515e07adde6edd45a7035b610))
* **api:** new streaming helpers for background responses ([422a0db](https://github.com/openai/openai-go/commit/422a0db3c674135e23dd200f5d8d785bd0be33e6))


### Chores

* **docs:** grammar improvements ([f4b23dd](https://github.com/openai/openai-go/commit/f4b23dd31facfc8839310854521b48060ef76be2))
* improve devcontainer setup ([dfdaeec](https://github.com/openai/openai-go/commit/dfdaeec2d6dd5cd679514d60c49b68c5df9e1b1e))

## 1.0.0 (2025-05-19)

Full Changelog: [v0.1.0-beta.11...v1.0.0](https://github.com/openai/openai-go/compare/v0.1.0-beta.11...v1.0.0)

### ⚠ BREAKING CHANGES

* **client:** rename file array param variant
* **api:** improve naming and remove assistants
* **accumulator:** update casing ([#401](https://github.com/openai/openai-go/issues/401))

### Features

* **api:** improve naming and remove assistants ([4c623b8](https://github.com/openai/openai-go/commit/4c623b88a9025db1961cc57985eb7374342f43e7))


### Bug Fixes

* **accumulator:** update casing ([#401](https://github.com/openai/openai-go/issues/401)) ([d59453c](https://github.com/openai/openai-go/commit/d59453c95b89fdd0b51305778dec0a39ce3a9d2a))
* **client:** correctly set stream key for multipart ([0ec68f0](https://github.com/openai/openai-go/commit/0ec68f0d779e7726931b1115eca9ae81eab59ba8))
* **client:** don't panic on marshal with extra null field ([9c15332](https://github.com/openai/openai-go/commit/9c153320272d212beaa516d4c70d54ae8053a958))
* **client:** increase max stream buffer size ([9456455](https://github.com/openai/openai-go/commit/945645559c5d68d9e28cf445d9c3b83e5fc6bd35))
* **client:** rename file array param variant ([4cfcf86](https://github.com/openai/openai-go/commit/4cfcf869280e7531fbbc8c00db0dd9271d07c423))
* **client:** use scanner for streaming ([aa58806](https://github.com/openai/openai-go/commit/aa58806bffc3aed68425c480414ddbb4dac3fa78))


### Chores

* **docs:** typo fix ([#400](https://github.com/openai/openai-go/issues/400)) ([bececf2](https://github.com/openai/openai-go/commit/bececf24cd0324b7c991b7d7f1d3eff6bf71f996))
* **examples:** migrate enum ([#447](https://github.com/openai/openai-go/issues/447)) ([814dd8b](https://github.com/openai/openai-go/commit/814dd8b6cfe4eeb535dc8ecd161a409ea2eb6698))
* **examples:** migrate to latest version ([#444](https://github.com/openai/openai-go/issues/444)) ([1c8754f](https://github.com/openai/openai-go/commit/1c8754ff905ed023f6381c8493910d63039407de))
* **examples:** remove beta assisstants examples ([#445](https://github.com/openai/openai-go/issues/445)) ([5891583](https://github.com/openai/openai-go/commit/589158372be9c0517b5508f9ccd872fdb1fe480b))
* **example:** update fine-tuning ([#450](https://github.com/openai/openai-go/issues/450)) ([421e3c5](https://github.com/openai/openai-go/commit/421e3c5065ace2d5ddd3d13a036477fff9123e5f))

## 0.1.0-beta.11 (2025-05-16)

Full Changelog: [v0.1.0-beta.10...v0.1.0-beta.11](https://github.com/openai/openai-go/compare/v0.1.0-beta.10...v0.1.0-beta.11)

### ⚠ BREAKING CHANGES

* **client:** clearer array variant names
* **client:** rename resp package
* **client:** improve core function names
* **client:** improve union variant names
* **client:** improve param subunions & deduplicate types

### Features

* **api:** add image sizes, reasoning encryption ([0852fb3](https://github.com/openai/openai-go/commit/0852fb3101dc940761f9e4f32875bfcf3669eada))
* **api:** add o3 and o4-mini model IDs ([3fabca6](https://github.com/openai/openai-go/commit/3fabca6b5c610edfb7bcd0cab5334a06444df0b0))
* **api:** Add reinforcement fine-tuning api support ([831a124](https://github.com/openai/openai-go/commit/831a12451cfce907b5ae4d294b9c2ac95f40d97a))
* **api:** adding gpt-4.1 family of model IDs ([1ef19d4](https://github.com/openai/openai-go/commit/1ef19d4cc94992dc435d7d5f28b30c9b1d255cd4))
* **api:** adding new image model support ([bf17880](https://github.com/openai/openai-go/commit/bf17880e182549c5c0fc34ec05df3184f223bc00))
* **api:** manual updates ([11f5716](https://github.com/openai/openai-go/commit/11f5716afa86aa100f80f3fa127e1d49203e5e21))
* **api:** responses x eval api ([183aaf7](https://github.com/openai/openai-go/commit/183aaf700f1d7ffad4ac847627d9ace65379c459))
* **api:** Updating Assistants and Evals API schemas ([47ca619](https://github.com/openai/openai-go/commit/47ca619fa1b439cf3a68c98e48e9bf1942f0568b))
* **client:** add dynamic streaming buffer to handle large lines ([8e6aad6](https://github.com/openai/openai-go/commit/8e6aad6d54fc73f1fcc174e1f06c9b3cf00c2689))
* **client:** add helper method to generate constant structs ([ff82809](https://github.com/openai/openai-go/commit/ff828094b561fc11184fed83f04424b6f68f7781))
* **client:** add support for endpoint-specific base URLs in python ([072dce4](https://github.com/openai/openai-go/commit/072dce46486d373fa0f0de5415f5270b01c2d972))
* **client:** add support for reading base URL from environment variable ([0d37268](https://github.com/openai/openai-go/commit/0d372687d673990290bad583f1906a2b121960b2))
* **client:** clearer array variant names ([a5d8b5d](https://github.com/openai/openai-go/commit/a5d8b5d6b161e3083184586840b2cbe0606d8de1))
* **client:** experimental support for unmarshalling into param structs ([5234875](https://github.com/openai/openai-go/commit/523487582e15a47e2f409f183568551258f4b8fe))
* **client:** improve param subunions & deduplicate types ([8a78f37](https://github.com/openai/openai-go/commit/8a78f37c25abf10498d16d210de3078f491ff23e))
* **client:** rename resp package ([4433516](https://github.com/openai/openai-go/commit/443351625ee290937a25425719b099ce785bd21b))
* **client:** support more time formats ([ec171b2](https://github.com/openai/openai-go/commit/ec171b2405c46f9cf04560760da001f7133d2fec))
* fix lint ([9c50a1e](https://github.com/openai/openai-go/commit/9c50a1eb9f93b578cb78085616f6bfab69f21dbc))


### Bug Fixes

* **client:** clean up reader resources ([710b92e](https://github.com/openai/openai-go/commit/710b92eaa7e94c03aeeca7479668677b32acb154))
* **client:** correctly update body in WithJSONSet ([f2d7118](https://github.com/openai/openai-go/commit/f2d7118295dd3073aa449426801d02e6f60bdaa3))
* **client:** improve core function names ([9f312a9](https://github.com/openai/openai-go/commit/9f312a9b14f5424d44d5834f1b82f3d3fcd57db2))
* **client:** improve union variant names ([a2c3de9](https://github.com/openai/openai-go/commit/a2c3de9e6c9f6e406b953f6de2eb78d1e72ec1b5))
* **client:** include path for type names in example code ([69561c5](https://github.com/openai/openai-go/commit/69561c549e18bd16a3641d62769479b125a4e955))
* **client:** resolve issue with optional multipart files ([910d173](https://github.com/openai/openai-go/commit/910d1730e97a03898e5dee7c889844a2ccec3e56))
* **client:** time format encoding fix ([ca17553](https://github.com/openai/openai-go/commit/ca175533ac8a17d36be1f531bbaa89c770da3f58))
* **client:** unmarshal responses properly ([fc9fec3](https://github.com/openai/openai-go/commit/fc9fec3c466ba9f633c3f7a4eebb5ebd3b85e8ac))
* handle empty bodies in WithJSONSet ([8372464](https://github.com/openai/openai-go/commit/83724640c6c00dcef1547dcabace309f17d14afc))
* **pagination:** handle errors when applying options ([eebf84b](https://github.com/openai/openai-go/commit/eebf84bf19f0eb6d9fa21e64bb83b0258e8cb42c))


### Chores

* **ci:** add timeout thresholds for CI jobs ([26b0dd7](https://github.com/openai/openai-go/commit/26b0dd760c142ca3aa287e8441bbe44cc8b3be0b))
* **ci:** only use depot for staging repos ([7682154](https://github.com/openai/openai-go/commit/7682154fdbcbe2a2ffdb2df590647a1712d52275))
* **ci:** run on more branches and use depot runners ([d7badbc](https://github.com/openai/openai-go/commit/d7badbc0d17bcf3cffec332f65cb68e531cb3176))
* **docs:** document pre-request options ([4befa5a](https://github.com/openai/openai-go/commit/4befa5a48ca61372715f36c45e72eb159d95bf2d))
* **docs:** update respjson package name ([9a00229](https://github.com/openai/openai-go/commit/9a002299a91e1145f053c51b1a4de10298fd2f43))
* **readme:** improve formatting ([a847e8d](https://github.com/openai/openai-go/commit/a847e8df45f725f9652fcea53ce57d3b9046efc7))
* **utils:** add internal resp to param utility ([239c4e2](https://github.com/openai/openai-go/commit/239c4e2cb32c7af71ab14668ccc2f52ea59653f9))


### Documentation

* update documentation links to be more uniform ([f5f0bb0](https://github.com/openai/openai-go/commit/f5f0bb05ee705d84119806f8e703bf2e0becb1fa))

## 0.1.0-beta.10 (2025-04-14)

Full Changelog: [v0.1.0-beta.9...v0.1.0-beta.10](https://github.com/openai/openai-go/compare/v0.1.0-beta.9...v0.1.0-beta.10)

### Chores

* **internal:** expand CI branch coverage ([#369](https://github.com/openai/openai-go/issues/369)) ([258dda8](https://github.com/openai/openai-go/commit/258dda8007a69b9c2720b225ee6d27474d676a93))
* **internal:** reduce CI branch coverage ([a2f7c03](https://github.com/openai/openai-go/commit/a2f7c03eb984d98f29f908df103ea1743f2e3d9a))

## 0.1.0-beta.9 (2025-04-09)

Full Changelog: [v0.1.0-beta.8...v0.1.0-beta.9](https://github.com/openai/openai-go/compare/v0.1.0-beta.8...v0.1.0-beta.9)

### Chores

* workaround build errors ([#366](https://github.com/openai/openai-go/issues/366)) ([adeb003](https://github.com/openai/openai-go/commit/adeb003cab8efbfbf4424e03e96a0f5e728551cb))

## 0.1.0-beta.8 (2025-04-09)

Full Changelog: [v0.1.0-beta.7...v0.1.0-beta.8](https://github.com/openai/openai-go/compare/v0.1.0-beta.7...v0.1.0-beta.8)

### Features

* **api:** Add evalapi to sdk ([#360](https://github.com/openai/openai-go/issues/360)) ([88977d1](https://github.com/openai/openai-go/commit/88977d1868dbbe0060c56ba5dac8eb19773e4938))
* **api:** manual updates ([#363](https://github.com/openai/openai-go/issues/363)) ([5d068e0](https://github.com/openai/openai-go/commit/5d068e0053172db7f5b75038aa215eee074eeeed))
* **client:** add escape hatch to omit required param fields ([#354](https://github.com/openai/openai-go/issues/354)) ([9690d6b](https://github.com/openai/openai-go/commit/9690d6b49f8b00329afc038ec15116750853e620))
* **client:** support custom http clients ([#357](https://github.com/openai/openai-go/issues/357)) ([b5a624f](https://github.com/openai/openai-go/commit/b5a624f658cad774094427b36b05e446b41e8c52))


### Chores

* **docs:** readme improvements ([#356](https://github.com/openai/openai-go/issues/356)) ([b2f8539](https://github.com/openai/openai-go/commit/b2f8539d6316e3443aa733be2c95926696119c13))
* **internal:** fix examples ([#361](https://github.com/openai/openai-go/issues/361)) ([de398b4](https://github.com/openai/openai-go/commit/de398b453d398299eb80c15f8fdb2bcbef5eeed6))
* **internal:** skip broken test ([#362](https://github.com/openai/openai-go/issues/362)) ([cccead9](https://github.com/openai/openai-go/commit/cccead9ba916142ac8fbe6e8926d706511e32ae3))
* **tests:** improve enum examples ([#359](https://github.com/openai/openai-go/issues/359)) ([e0b9739](https://github.com/openai/openai-go/commit/e0b9739920114d6e991d3947b67fdf62cfaa09c7))

## 0.1.0-beta.7 (2025-04-07)

Full Changelog: [v0.1.0-beta.6...v0.1.0-beta.7](https://github.com/openai/openai-go/compare/v0.1.0-beta.6...v0.1.0-beta.7)

### Features

* **client:** make response union's AsAny method type safe ([#352](https://github.com/openai/openai-go/issues/352)) ([1252f56](https://github.com/openai/openai-go/commit/1252f56c917e57d6d2b031501b2ff5f89f87cf87))


### Chores

* **docs:** doc improvements ([#350](https://github.com/openai/openai-go/issues/350)) ([80debc8](https://github.com/openai/openai-go/commit/80debc824eaacb4b07c8f3e8b1d0488d860d5be5))

## 0.1.0-beta.6 (2025-04-04)

Full Changelog: [v0.1.0-beta.5...v0.1.0-beta.6](https://github.com/openai/openai-go/compare/v0.1.0-beta.5...v0.1.0-beta.6)

### Features

* **api:** manual updates ([4e39609](https://github.com/openai/openai-go/commit/4e39609d499b88039f1c90cc4b56e26f28fd58ea))
* **client:** support unions in query and forms ([#347](https://github.com/openai/openai-go/issues/347)) ([cf8af37](https://github.com/openai/openai-go/commit/cf8af373ab7c019c75e886855009ffaca320d0e3))

## 0.1.0-beta.5 (2025-04-03)

Full Changelog: [v0.1.0-beta.4...v0.1.0-beta.5](https://github.com/openai/openai-go/compare/v0.1.0-beta.4...v0.1.0-beta.5)

### Features

* **api:** manual updates ([563cc50](https://github.com/openai/openai-go/commit/563cc505f2ab17749bb77e937342a6614243b975))
* **client:** omitzero on required id parameter ([#339](https://github.com/openai/openai-go/issues/339)) ([c0b4842](https://github.com/openai/openai-go/commit/c0b484266ccd9faee66873916d8c0c92ea9f1014))


### Bug Fixes

* **client:** return error on bad custom url instead of panic ([#341](https://github.com/openai/openai-go/issues/341)) ([a06c5e6](https://github.com/openai/openai-go/commit/a06c5e632242e53d3fdcc8964931acb533a30b7e))
* **client:** support multipart encoding array formats ([#342](https://github.com/openai/openai-go/issues/342)) ([5993b28](https://github.com/openai/openai-go/commit/5993b28309d02c2d748b54d98934ef401dcd193a))
* **client:** unmarshal stream events into fresh memory ([#340](https://github.com/openai/openai-go/issues/340)) ([52c3e08](https://github.com/openai/openai-go/commit/52c3e08f51d471d728e5acd16b3c304b51be2d03))

## 0.1.0-beta.4 (2025-04-02)

Full Changelog: [v0.1.0-beta.3...v0.1.0-beta.4](https://github.com/openai/openai-go/compare/v0.1.0-beta.3...v0.1.0-beta.4)

### Features

* **api:** manual updates ([bc4fe73](https://github.com/openai/openai-go/commit/bc4fe73eec9c4d39229e4beae8eaafb55b1d3364))
* **api:** manual updates ([aa7ff10](https://github.com/openai/openai-go/commit/aa7ff10b0616a6b2ece45cb10e9c83f25e35aded))


### Chores

* **docs:** update file uploads in README ([#333](https://github.com/openai/openai-go/issues/333)) ([471c452](https://github.com/openai/openai-go/commit/471c4525c94e83cf4b78cb6c9b2f65a8a27bf3ce))
* **internal:** codegen related update ([#335](https://github.com/openai/openai-go/issues/335)) ([48422dc](https://github.com/openai/openai-go/commit/48422dcca333ab808ccb02506c033f1c69d2aa19))
* Remove deprecated/unused remote spec feature ([c5077a1](https://github.com/openai/openai-go/commit/c5077a154a6db79b73cf4978bdc08212c6da6423))

## 0.1.0-beta.3 (2025-03-28)

Full Changelog: [v0.1.0-beta.2...v0.1.0-beta.3](https://github.com/openai/openai-go/compare/v0.1.0-beta.2...v0.1.0-beta.3)

### ⚠ BREAKING CHANGES

* **client:** add enums ([#327](https://github.com/openai/openai-go/issues/327))

### Features

* **api:** add `get /chat/completions` endpoint ([e8ed116](https://github.com/openai/openai-go/commit/e8ed1168576c885cb26fbf819b9c8d24975749bd))
* **api:** add `get /responses/{response_id}/input_items` endpoint ([8870c26](https://github.com/openai/openai-go/commit/8870c26f010a596adcf37ac10dba096bdd4394e3))


### Bug Fixes

* **client:** add enums ([#327](https://github.com/openai/openai-go/issues/327)) ([b0e3afb](https://github.com/openai/openai-go/commit/b0e3afbd6f18fd9fc2a5ea9174bd7ec0ac0614db))


### Chores

* add hash of OpenAPI spec/config inputs to .stats.yml ([104b786](https://github.com/openai/openai-go/commit/104b7861bb025514999b143f7d1de45d2dab659f))
* add request options to client tests ([#321](https://github.com/openai/openai-go/issues/321)) ([f5239ce](https://github.com/openai/openai-go/commit/f5239ceecf36835341eac5121ed1770020c4806a))
* **api:** updates to supported Voice IDs ([#325](https://github.com/openai/openai-go/issues/325)) ([477727a](https://github.com/openai/openai-go/commit/477727a44b0fb72493c4749cc60171e0d30f98ec))
* **docs:** improve security documentation ([#319](https://github.com/openai/openai-go/issues/319)) ([0271053](https://github.com/openai/openai-go/commit/027105363ab30ac3e189234908169faf94e0ca49))
* fix typos ([#324](https://github.com/openai/openai-go/issues/324)) ([dba15f7](https://github.com/openai/openai-go/commit/dba15f74d63814ce16f778e1017a209a42f46179))

## 0.1.0-beta.2 (2025-03-22)

Full Changelog: [v0.1.0-beta.1...v0.1.0-beta.2](https://github.com/openai/openai-go/compare/v0.1.0-beta.1...v0.1.0-beta.2)

### Bug Fixes

* **client:** elide fields in ToAssistantParam ([#309](https://github.com/openai/openai-go/issues/309)) ([1fcd837](https://github.com/openai/openai-go/commit/1fcd83753ea806745d278a5b94797bbee0f018ed))

## 0.1.0-beta.1 (2025-03-22)

Full Changelog: [v0.1.0-alpha.67...v0.1.0-beta.1](https://github.com/openai/openai-go/compare/v0.1.0-alpha.67...v0.1.0-beta.1)

### Chores

* **docs:** clarify breaking changes ([#306](https://github.com/openai/openai-go/issues/306)) ([db4bd1f](https://github.com/openai/openai-go/commit/db4bd1f5304aa523a6b62da6e2571487d4248518))

## 0.1.0-alpha.67 (2025-03-21)

Full Changelog: [v0.1.0-alpha.66...v0.1.0-alpha.67](https://github.com/openai/openai-go/compare/v0.1.0-alpha.66...v0.1.0-alpha.67)

### ⚠ BREAKING CHANGES

* **api:** migrate to v2

### Features

* **api:** migrate to v2 ([9377508](https://github.com/openai/openai-go/commit/9377508e45ae485d11c3199d6d3d91d345f1b76e))
* **api:** new models for TTS, STT, + new audio features for Realtime ([#298](https://github.com/openai/openai-go/issues/298)) ([48fa064](https://github.com/openai/openai-go/commit/48fa064202a6e4a3e850d435b29f6fe9a1fe53f4))


### Chores

* **internal:** bugfix ([0d8c1f4](https://github.com/openai/openai-go/commit/0d8c1f4e801785728b6ad3342146fe38874d6c04))


### Documentation

* add migration guide ([#302](https://github.com/openai/openai-go/issues/302)) ([19e32fa](https://github.com/openai/openai-go/commit/19e32fa595e65048bb129e813c697991117abca2))
