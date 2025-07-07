# Changelog

## [1.3.0](https://github.com/googleapis/go-genai/compare/v1.2.0...v1.3.0) (2025-04-30)


### Features

* add models.delete and models.update to manage tuned models ([6f5bbed](https://github.com/googleapis/go-genai/commit/6f5bbed7ab1514d246befe4e8aa16c4244678b25))
* add NewPartFromFile for File type convenience. Related to [#281](https://github.com/googleapis/go-genai/issues/281) ([2ac0429](https://github.com/googleapis/go-genai/commit/2ac0429c634edca2396c272f27f013b4a960529a))
* add support for live grounding metadata ([9ce2ed9](https://github.com/googleapis/go-genai/commit/9ce2ed91ffbe781b513487ca29b86d95497e8f06))
* make min_property, max_property, min_length, max_length, example, patter fields available for Schema class when calling Gemini API ([b487724](https://github.com/googleapis/go-genai/commit/b48772435585584c9974b923d071fe567cd00366))
* Populate X-Server-Timeout header when a request timeout is set. ([8f446a0](https://github.com/googleapis/go-genai/commit/8f446a0a0c5ffe74ec00e32b0667d830596db49d))
* Support setting the default base URL in clients via SetDefaultBaseURLs() ([f465a20](https://github.com/googleapis/go-genai/commit/f465a2088c0d4a104f4edbd7197b64f1097fc1b8))


### Bug Fixes

* do not raise error for `default` field in Schema for Gemini API calls ([ec31e4b](https://github.com/googleapis/go-genai/commit/ec31e4b92afca2f167456ef5e8f775cfad198b8d))
* do not remove content parts with `Text` unset. ([b967057](https://github.com/googleapis/go-genai/commit/b967057d68ad8cd5385aa19b65b0648646cb8c00))
* **files:** deep copy config struct before modifying it. ([a6b0fd6](https://github.com/googleapis/go-genai/commit/a6b0fd6c47cfda6ce28d01a8119ae6f38e2214f4))

## [1.2.0](https://github.com/googleapis/go-genai/compare/v1.1.0...v1.2.0) (2025-04-23)


### Features

* add additional realtime input fields ([1190539](https://github.com/googleapis/go-genai/commit/11905391266e0ee01e7eff4ee68304bcc3654f36))
* Expose transcription configurations for audio in TS, and move generationConfig to the top level LiveConnectConfig ([ead7e49](https://github.com/googleapis/go-genai/commit/ead7e49e4e710a31bd87cf18d0f7d5925bae8662))
* support `default` field in Schema when users call Gemini API ([643eb80](https://github.com/googleapis/go-genai/commit/643eb801b141ab27e05d77678f179c9a1dc5407c))

## [1.1.0](https://github.com/googleapis/go-genai/compare/v1.0.0...v1.1.0) (2025-04-16)


### Features

* Add converters to support continuous sessions with a sliding window ([97bbba4](https://github.com/googleapis/go-genai/commit/97bbba4655ec1b754532c10d464ed037e7312158))
* add support for model_selection_config to GenerateContentConfig ([a44a6c7](https://github.com/googleapis/go-genai/commit/a44a6c78bb4e2bc614bcf526930338e1fd6b84a7))
* Add types for configurable speech detection ([658c17a](https://github.com/googleapis/go-genai/commit/658c17a226256decb52c59e595d360867cf987ea))
* Support audio transcription in Vertex Live API ([f09dfab](https://github.com/googleapis/go-genai/commit/f09dfab7de4a8bb4abf52302b0df3749a7e043a6))
* Support RealtimeInputConfig, and language_code in SpeechConfig in python ([f90a0ec](https://github.com/googleapis/go-genai/commit/f90a0ec3bd58759152b5c417c9d725cedbb98fe9))
* Update VertexRagStore ([62d582c](https://github.com/googleapis/go-genai/commit/62d582c9f2a0bd4149f6a87efd3b71886fa969ef))


### Bug Fixes

* **files:** use `io.ReadFull` to read the file. fixes [#237](https://github.com/googleapis/go-genai/issues/237) ([908c783](https://github.com/googleapis/go-genai/commit/908c78371f2c9e16b4bd43286b6e7ed95d02fa8e))
* Fix error "assignment to entry in nil map" of `Files.Upload()` when `config.HTTPOptions` is nil ([#235](https://github.com/googleapis/go-genai/issues/235)) ([05c0c49](https://github.com/googleapis/go-genai/commit/05c0c49512f56dd808408ddf555699af6b164ac3))
* fix MIME type error in UploadFromPath and add unit tests. fixes: [#247](https://github.com/googleapis/go-genai/issues/247) ([f851639](https://github.com/googleapis/go-genai/commit/f851639a7f5bc3d8392a8d2cee2e25ed0d42feda))

## [1.0.0](https://github.com/googleapis/go-genai/compare/v0.7.0...v1.0.0) (2025-04-09)


### ⚠ BREAKING CHANGES

* Support SendClientContent/SendRealtimeInput/SendToolResponse methods in Session struct and remove Send method
* Merge GenerationConfig to LiveConnectConfig. GenerationConfig is removed.
* Change NewContentFrom... functions role param type from string to Role and miscs docstring improvements
* Change some pointer to value type and value to pointer type

### Features

* Add domain to Web GroundingChunk ([183ac49](https://github.com/googleapis/go-genai/commit/183ac49d75bb8a84c95df6aba6b284761509e61e))
* Add generationComplete notification to Live ServerContent ([9a038b9](https://github.com/googleapis/go-genai/commit/9a038b96cc8e979649033a6636387329da443b26))
* Add session resumption to Live module ([4a92461](https://github.com/googleapis/go-genai/commit/4a92461832b60b7a4adf32d99f3a50651c4db50b))
* add session resumption. ([507137b](https://github.com/googleapis/go-genai/commit/507137bcbe76e8e2b4a7372038e3136fb4a36425))
* Add support for Chats streaming in Go SDK ([9ee0523](https://github.com/googleapis/go-genai/commit/9ee0523e4975ddced4b3918ada8bdea4c1a0787f))
* Add thinking_budget to ThinkingConfig for Gemini Thinking Models ([f811ee4](https://github.com/googleapis/go-genai/commit/f811ee48b67db553b7520bc417f366270415d95e))
* Add traffic type to GenerateContentResponseUsageMetadata ([601add2](https://github.com/googleapis/go-genai/commit/601add239ae6722ab84f9bfabe3b0d4a84bf7b42))
* Add types for configurable speech detection ([f4e1b11](https://github.com/googleapis/go-genai/commit/f4e1b118df97866e8b7b47baedde9470cb842ed0))
* Add types to support continuous sessions with a sliding window ([5d4f5d7](https://github.com/googleapis/go-genai/commit/5d4f5d7e5e3ce96f7876fc8a65ef49c5c796a6ad))
* Add UsageMetadata to LiveServerMessage ([4286c6b](https://github.com/googleapis/go-genai/commit/4286c6bf04adee388c4dcdc83c4fe5923558b573))
* expose generation_complete, input/output_transcription & input/output_audio_transcription to SDK for Vertex Live API ([0dbbc82](https://github.com/googleapis/go-genai/commit/0dbbc82a0f03c617d01726468993c58128016dca))
* Merge GenerationConfig to LiveConnectConfig. GenerationConfig is removed. ([65b7c1c](https://github.com/googleapis/go-genai/commit/65b7c1c51e6d954f3c2d61202f6d7b6ba5a8ceb1))
* Remove experimental warnings for generate_videos and operations ([2e4bb0b](https://github.com/googleapis/go-genai/commit/2e4bb0bb12f2eb3a88d4d125ed8bc6c8166e051f))
* Support files delete, get, list, download/ ([8e7b3fd](https://github.com/googleapis/go-genai/commit/8e7b3fd50775ab4ca11484a85a40166066e05f6a))
* Support files upload method ([ce790dd](https://github.com/googleapis/go-genai/commit/ce790ddd9b34c12c913634b890ba5fa01f86c18a))
* support media resolution ([825c81d](https://github.com/googleapis/go-genai/commit/825c81dbcb9eeff54f52052270e1f5d738fab39c))
* Support SendClientContent/SendRealtimeInput/SendToolResponse methods in Session struct and remove Send method ([c8ecaf4](https://github.com/googleapis/go-genai/commit/c8ecaf4ffa2c3f5ca59692af6711651966630729))
* use io.Reader in Upload function and add a new convenience function UploadFromPath. fixes [#222](https://github.com/googleapis/go-genai/issues/222) ([1c064e3](https://github.com/googleapis/go-genai/commit/1c064e3e15c75e987189cb4a65080a4aa087531d))


### Bug Fixes

* Change NewContentFrom... functions role param type from string to Role and miscs docstring improvements ([7810e07](https://github.com/googleapis/go-genai/commit/7810e074299bbd9c38160a995cc6df311a3e9e88))
* Change some pointer to value type and value to pointer type ([0d2ba97](https://github.com/googleapis/go-genai/commit/0d2ba97b813ad51f964306de4399cbdd777105eb))
* fix Add() dead loop ([afa2324](https://github.com/googleapis/go-genai/commit/afa23240ac30a0fafca7877d5034f34a3c187e91))
* Fix failing chat_test ([aebbdaa](https://github.com/googleapis/go-genai/commit/aebbdaa234b2a0552f738c593a46094e6016dedc))

## [0.7.0](https://github.com/googleapis/go-genai/compare/v0.6.0...v0.7.0) (2025-03-31)


### ⚠ BREAKING CHANGES

* Add error return type to Close() function
* consolidate NewUserContentFrom* and NewModelContentFrom* functions into NewContentFrom* to make API simpler
* Support quota project and migrate ClientConfig.Credential from google.Credentials to auth.Credential type.
* Change caches TTL field to duration type.
* rename ClientError and ServerError to APIError. fixes: #159

### Features

* Add Chats module for Go SDK (non-stream only) ([e7f75fd](https://github.com/googleapis/go-genai/commit/e7f75fdd931001e5e3e68c453201ce933a70f064))
* Add engine to VertexAISearch ([cc2ab5d](https://github.com/googleapis/go-genai/commit/cc2ab5dc7013f045d6d7393cc7cbd05988f767da))
* add IMAGE_SAFTY enum value to FinishReason ([cc6081a](https://github.com/googleapis/go-genai/commit/cc6081a7e781fb68a6cbcb89528de85c31c4fb6a))
* add MediaModalities for ModalityTokenCount ([0969afd](https://github.com/googleapis/go-genai/commit/0969afd3854fdec86e001f3412582aa95123286f))
* Add Veo 2 generate_videos support in Go SDK ([5321a25](https://github.com/googleapis/go-genai/commit/5321a25f0134b8b2d45ebdfb73544123044f96c7))
* allow title property to be sent to Gemini API. Gemini API now supports the title property, so it's ok to pass this onto both Vertex and Gemini API. ([8f27aba](https://github.com/googleapis/go-genai/commit/8f27aba6199bfac6205fb7e88883a5c6a1ee017e))
* consolidate NewUserContentFrom* and NewModelContentFrom* functions into NewContentFrom* to make API simpler ([e8608b1](https://github.com/googleapis/go-genai/commit/e8608b19f7bec5cb976095b2d5cdb69886ae6036))
* merge GenerationConfig into LiveConnectConfig ([96232de](https://github.com/googleapis/go-genai/commit/96232de67aa69af0f1e10625961765b13d3dbfc5))
* rename ClientError and ServerError to APIError. fixes: [#159](https://github.com/googleapis/go-genai/issues/159) ([12adbfa](https://github.com/googleapis/go-genai/commit/12adbfae781a1df63a32094895dc0b37baad32da))
* Save prompt safety attributes in dedicated field for generate_images ([eb3cfdc](https://github.com/googleapis/go-genai/commit/eb3cfdc8773b85bae648a90dddaa69435824a58b))
* support new UsageMetadata fields ([3a56c63](https://github.com/googleapis/go-genai/commit/3a56c632f11d703786cb546d4ced3ee7bbf84b39))
* Support quota project and migrate ClientConfig.Credential from google.Credentials to auth.Credential type. ([74c05fb](https://github.com/googleapis/go-genai/commit/74c05fbf68e3c35627d69720d3de733f0d38cbce))


### Bug Fixes

* Add error return type to Close() function ([673a7f7](https://github.com/googleapis/go-genai/commit/673a7f7e61cf4a3377e145d4aed8f54b7d90886f))
* Change caches TTL field to duration type. ([11271b4](https://github.com/googleapis/go-genai/commit/11271b4d888741d5dcaebbe9dea44daface9e198))
* fix list models API url ([036c4d3](https://github.com/googleapis/go-genai/commit/036c4d3e368c1184641e9e089056b57c875e2a10))
* fix response modality in streaming mode. fixes [#163](https://github.com/googleapis/go-genai/issues/163). fixes [#158](https://github.com/googleapis/go-genai/issues/158) ([996dac3](https://github.com/googleapis/go-genai/commit/996dac39f23dff4436dfea3f2badf414f9435338))
* missing zero value bug in setValueByPath. fixes [#196](https://github.com/googleapis/go-genai/issues/196) ([557c6d8](https://github.com/googleapis/go-genai/commit/557c6d8a8de80caf6999fc2ba2be166e140e8880))
* schema transformer logic fix. ([8017092](https://github.com/googleapis/go-genai/commit/8017092b7cfe42a44e5b4b09f4c934ac723618f4))
* use snake_case in embed_content request/response parsing. fixes [#174](https://github.com/googleapis/go-genai/issues/174) ([ba644e1](https://github.com/googleapis/go-genai/commit/ba644e19b03d948487da3b12f843fe32cb3b1851))


### Miscellaneous Chores

* release 0.7.0 ([06523b4](https://github.com/googleapis/go-genai/commit/06523b4d9b90c3dae5dba72331297c5c1d23e28d))

## [0.6.0](https://github.com/googleapis/go-genai/compare/v0.5.0...v0.6.0) (2025-03-19)


### ⚠ BREAKING CHANGES

* support duration type and remove NewPartFromVideoMetadata function
* Change *time.Time type to time.Time.
* remove error from the GenerateContentResponse.Text() return values and add more samples(text embedding, tokens, models)
* change GenerateImageConfig.NumberOfImages to value type. And add clearer error message and docstring to other APIs.
* Remove default role to "user" for GenerateContent and GenerateContentStream.

### Features

* Add base steps to EditImageConfig ([e3c8252](https://github.com/googleapis/go-genai/commit/e3c82523429d43684e898a10991fb86161f5f48f))
* Change *time.Time type to time.Time. ([d554a08](https://github.com/googleapis/go-genai/commit/d554a081fff30d0fec4395ef5d8dd936d81a5477))
* change GenerateImageConfig.NumberOfImages to value type. And add clearer error message and docstring to other APIs. ([a75a9ae](https://github.com/googleapis/go-genai/commit/a75a9ae4d7f782c8894b9c8bc7e9c44f93e71fe6))
* enable union type for Schema when calling Gemini API. ([2edcc55](https://github.com/googleapis/go-genai/commit/2edcc5560a89b76542d77566890911bf1a163795))
* Remove default role to "user" for GenerateContent and GenerateContentStream. ([74d4647](https://github.com/googleapis/go-genai/commit/74d46476678813c1888d89b0112c94f6fa0d3a2e))
* remove error from the GenerateContentResponse.Text() return values and add more samples(text embedding, tokens, models) ([1dc5c1c](https://github.com/googleapis/go-genai/commit/1dc5c1c95acb2f207632eeeeb8fa6d4cbb6a7df4))
* support duration type and remove NewPartFromVideoMetadata function ([b2a13ab](https://github.com/googleapis/go-genai/commit/b2a13ab16cfbd6b167d2541128a75ab059ffc044))
* Support global endpoint in go natively ([a29b806](https://github.com/googleapis/go-genai/commit/a29b806d89dd7ebddb44486c23cd51f79864029d))
* Support returned safety attributes for generate_images ([cc2bf1a](https://github.com/googleapis/go-genai/commit/cc2bf1aa581439b2d674966eed55caa580038a83))


### Bug Fixes

* Make month and day optional for PublicationDate. fixes [#141](https://github.com/googleapis/go-genai/issues/141) ([8a61516](https://github.com/googleapis/go-genai/commit/8a615165d2161f5be0efb0d7bf5f77570166b0b0))
* Remove unsupported parameter negative_prompt from Gemini API generate_images ([be2619d](https://github.com/googleapis/go-genai/commit/be2619d6d2304f680ae8f9b2b669a6799929988b))


### Miscellaneous Chores

* release 0.6.0 ([f636767](https://github.com/googleapis/go-genai/commit/f636767b3fdc4c4a186c9465fdc3cb2d950c158b))

## [0.5.0](https://github.com/googleapis/go-genai/compare/v0.4.0...v0.5.0) (2025-03-06)


### ⚠ BREAKING CHANGES

* change int64, float64 types to int32, unit32, float32 to prevent data loss
* remove ClientConfig.Timeout and add HTTPOptions to ...Config structs

### Features

* Add Headers field into HTTPOption struct ([5ec9ff4](https://github.com/googleapis/go-genai/commit/5ec9ff40ce4e9f3fd4625eab68dfbe5e9d259237))
* Add response_id and create_time to GenerateContentResponse ([f46d996](https://github.com/googleapis/go-genai/commit/f46d9969fe228dfa8703224fe36c2fcc8cd6540d))
* added Models.list() function ([6c2eae4](https://github.com/googleapis/go-genai/commit/6c2eae47aa6fb60cd2f6ae52744033359e0093ba))
* enable minItem, maxItem, nullable for Schema type when calling Gemini API. ([fb6c8a5](https://github.com/googleapis/go-genai/commit/fb6c8a528b195f07dae7b6130eee059a40d35803))
* enable quick accessor of executable code and code execution result in GenerateContentResponse ([21ca251](https://github.com/googleapis/go-genai/commit/21ca2516b27cbf51b4ab3486da9ca31f3a908204))
* remove ClientConfig.Timeout and add HTTPOptions to ...Config structs ([ba6c431](https://github.com/googleapis/go-genai/commit/ba6c43132ce8a2fcad1fdad48bc3f80b6ecb0a96))
* Support aspect ratio for edit_image ([06d554f](https://github.com/googleapis/go-genai/commit/06d554f78ce4b61cc113f5254c4f5b48415ce25e))
* support edit image and add sample for imagen ([f332cf2](https://github.com/googleapis/go-genai/commit/f332cf26e0c570cd2af4e797a01930ea55b096eb))
* Support Models.EmbedContent function ([a71f0a7](https://github.com/googleapis/go-genai/commit/a71f0a7a181181316e02f4fe21ad6acddae68c1b))


### Bug Fixes

* change int64, float64 types to int32, unit32, float32 to prevent data loss ([af83fa7](https://github.com/googleapis/go-genai/commit/af83fa7501b3e81102b35c1bffd76cdf68203d1b))
* log warning instead of throwing error for GenerateContentResponse.text() quick accessor when there are mixed types of parts. ([006e3af](https://github.com/googleapis/go-genai/commit/006e3af99fb568d89926bb6129b8d890e8f6a0db))


### Miscellaneous Chores

* release 0.5.0 ([14bdd8f](https://github.com/googleapis/go-genai/commit/14bdd8f9b7148c2aa588249415c29396c3b6217c))

## [0.4.0](https://github.com/googleapis/go-genai/compare/v0.3.0...v0.4.0) (2025-02-24)


### Features

* Add Imagen upscale_image support for Go ([8e2afe9](https://github.com/googleapis/go-genai/commit/8e2afe992bae5b30c6d9cd2bfecfc71f12c3f986))
* introduce usability functions to allow quick creation of user content and model content. ([12b5dee](https://github.com/googleapis/go-genai/commit/12b5dee0e6148aa00c5ee3516189e79dc07b1ab8))
* support list all caches in List and All functions ([addc388](https://github.com/googleapis/go-genai/commit/addc3880e38c6026117d91f8019959347469ef12))
* support Models .Get, .Update, .Delete ([e67cd8b](https://github.com/googleapis/go-genai/commit/e67cd8b2d619323bfce97a3b6306521799a6b4f9))


### Bug Fixes

* fix the civil.Date parsing in Citation struct. fixes [#106](https://github.com/googleapis/go-genai/issues/106) ([f530fcf](https://github.com/googleapis/go-genai/commit/f530fcf86fec626bd6bad88c72d26746acada4ff))
* missing context in request. fixes [#104](https://github.com/googleapis/go-genai/issues/104) ([747c5ef](https://github.com/googleapis/go-genai/commit/747c5ef9c781024b0f88f30c77ff382b35f6a52b))
* Remove request body when it's empty. ([cfc82e3](https://github.com/googleapis/go-genai/commit/cfc82e3ca5231506172c9258a1447a114a84ed96))

## [0.3.0](https://github.com/googleapis/go-genai/compare/v0.2.0...v0.3.0) (2025-02-12)


### Features

* Enable Media resolution for Gemini API. ([a22788b](https://github.com/googleapis/go-genai/commit/a22788bb061458bbd15c2fd1a8e2dfdf9e7a3fc8))
* support property_ordering in response_schema (fixes [#236](https://github.com/googleapis/go-genai/issues/236)) ([ac45038](https://github.com/googleapis/go-genai/commit/ac450381046cd673d6a76e04920fc610b182c2c0))

## [0.2.0](https://github.com/googleapis/go-genai/compare/v0.1.0...v0.2.0) (2025-02-05)


### Features

* Add enhanced_prompt to GeneratedImage class ([449f0fb](https://github.com/googleapis/go-genai/commit/449f0fbc1f57b5ce5e20eef587f67f2d0d93a889))
* Add labels for GenerateContent requests ([98231e5](https://github.com/googleapis/go-genai/commit/98231e5e7fa2483004841b50ceee841078e6d951))


### Bug Fixes

* remove unsupported parameter from Gemini API ([39c8868](https://github.com/googleapis/go-genai/commit/39c88682acbf554bad4d7a8ca92a854a7005052a))
* Use camel case for Go function parameters ([94765e6](https://github.com/googleapis/go-genai/commit/94765e68aef1258054711cc601e070e4ef7c80e5))

## [0.1.0](https://github.com/googleapis/go-genai/compare/v0.0.1...v0.1.0) (2025-01-29)


### ⚠ BREAKING CHANGES

* Make some numeric fields to pointer type and bool fields to value type, and rename ControlReferenceTypeControlType* constants

### Features

* [genai-modules][models] Add HttpOptions to all method configs for models. ([765c9b7](https://github.com/googleapis/go-genai/commit/765c9b7311884554c352ec00a0253c2cbbbf665c))
* Add Imagen generate_image support for Go SDK ([068fe54](https://github.com/googleapis/go-genai/commit/068fe541801ced806714662af023a481271402c4))
* Add support for audio_timestamp to types.GenerateContentConfig (fixes [#132](https://github.com/googleapis/go-genai/issues/132)) ([cfede62](https://github.com/googleapis/go-genai/commit/cfede6255a13b4977450f65df80b576342f44b5a))
* Add support for enhance_prompt to model.generate_image ([a35f52a](https://github.com/googleapis/go-genai/commit/a35f52a318a874935a1e615dbaa24bb91625c5de))
* Add ThinkingConfig to generate content config. ([ad73778](https://github.com/googleapis/go-genai/commit/ad73778cf6f1c6d9b240cf73fce52b87ae70378f))
* enable Text() and FunctionCalls() quick accessor for GenerateContentResponse ([3f3a450](https://github.com/googleapis/go-genai/commit/3f3a450954283fa689c9c19a29b0487c177f7aeb))
* Images - Added Image.mime_type ([3333511](https://github.com/googleapis/go-genai/commit/3333511a656b796065cafff72168c112c74de293))
* introducing HTTPOptions to Client ([e3d1d8e](https://github.com/googleapis/go-genai/commit/e3d1d8e6aa0cbbb3f2950c571f5c0a70b7ce8656))
* make Part, FunctionDeclaration, Image, and GenerateContentResponse classmethods argument keyword only ([f7d1043](https://github.com/googleapis/go-genai/commit/f7d1043bb791930d82865a11b83fea785e313922))
* Make some numeric fields to pointer type and bool fields to value type, and rename ControlReferenceTypeControlType* constants ([ee4e5a4](https://github.com/googleapis/go-genai/commit/ee4e5a414640226e9b685a7d67673992f2c63dee))
* support caches create/update/get/update in Go SDK ([0620d97](https://github.com/googleapis/go-genai/commit/0620d97e32b3e535edab8f3f470e08746ace4d60))
* support usability constructor functions for Part struct ([831b879](https://github.com/googleapis/go-genai/commit/831b879ea15a82506299152e9f790f34bbe511f9))


### Miscellaneous Chores

* Released as 0.1.0 ([e046125](https://github.com/googleapis/go-genai/commit/e046125c8b378b5acb05e64ed46c4aac51dd9456))


### Code Refactoring

* rename GenerateImage() to GenerateImage(), rename GenerateImageConfig to GenerateImagesConfig, rename GenerateImageResponse to GenerateImagesResponse, rename GenerateImageParameters to GenerateImagesParameters ([ebb231f](https://github.com/googleapis/go-genai/commit/ebb231f0c86bb30f013301e26c562ccee8380ee0))

## 0.0.1 (2025-01-10)


### Features

* enable response_logprobs and logprobs for Google AI ([#17](https://github.com/googleapis/go-genai/issues/17)) ([51f2744](https://github.com/googleapis/go-genai/commit/51f274411ea770fa8fc16ce316085310875e5d68))
* Go SDK Live module implementation for GoogleAI backend ([f88e65a](https://github.com/googleapis/go-genai/commit/f88e65a7f8fda789b0de5ecc4e2ed9d2bd02cc89))
* Go SDK Live module initial implementation for VertexAI. ([4d82dc0](https://github.com/googleapis/go-genai/commit/4d82dc0c478151221d31c0e3ccde9ac215f2caf2))


### Bug Fixes

* change string type to numeric types ([bfdc94f](https://github.com/googleapis/go-genai/commit/bfdc94fd1b38fb61976f0386eb73e486cc3bc0f8))
* fix README typo ([5ae8aa6](https://github.com/googleapis/go-genai/commit/5ae8aa6deec520f33d1746be411ed55b2b10d74f))
