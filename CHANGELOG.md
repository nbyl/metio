# Changelog

## [1.6.2](https://github.com/nbyl/metio/compare/v1.6.1...v1.6.2) (2026-08-10)


### Bug Fixes

* provider settings screwed by the AI ([c8d7d64](https://github.com/nbyl/metio/commit/c8d7d64c55f211d0a0916df37ea6b1950fe6ea38))
* provider settings screwed by the AI ([4c64d3a](https://github.com/nbyl/metio/commit/4c64d3afb1cb3c454aa3782a0344b185e41514ff))

## [1.6.1](https://github.com/nbyl/metio/compare/v1.6.0...v1.6.1) (2026-08-10)


### Bug Fixes

* **deploy:** drop write-only attributes for OpenTofu &lt;1.11 compatibility ([ddbf4e9](https://github.com/nbyl/metio/commit/ddbf4e97a29c4e56bd57844c9d39ae31de29467b))
* **deploy:** drop write-only attributes for OpenTofu &lt;1.11 compatibility ([4851d51](https://github.com/nbyl/metio/commit/4851d518e370a421516b12b66c19106684931b0f)), closes [#351](https://github.com/nbyl/metio/issues/351)

## [1.6.0](https://github.com/nbyl/metio/compare/v1.5.0...v1.6.0) (2026-08-06)


### Features

* add Dapr sidecar infrastructure for Cloud Run controller ([#255](https://github.com/nbyl/metio/issues/255)) ([66e1621](https://github.com/nbyl/metio/commit/66e162134596bb11e7a73cf1927fc135c617d876))
* add Dapr sidecar infrastructure for Cloud Run controller ([#255](https://github.com/nbyl/metio/issues/255)) ([519c0a3](https://github.com/nbyl/metio/commit/519c0a32a8b8a297fc80adbdf3ed3aca55303740))
* add DB_BACKEND toggle for Dapr state store selection ([#257](https://github.com/nbyl/metio/issues/257)) ([2fa09d8](https://github.com/nbyl/metio/commit/2fa09d8099ed8cb9146843e543933d3b4cb18eac))
* add DB_BACKEND toggle for Dapr state store selection ([#257](https://github.com/nbyl/metio/issues/257)) ([6020911](https://github.com/nbyl/metio/commit/60209112a04069152bc379a74db0b4da83427dd8))
* add JSON tags and Dapr key schema for state store migration ([#258](https://github.com/nbyl/metio/issues/258)) ([eadc24d](https://github.com/nbyl/metio/commit/eadc24d27f652b011612365c5d28e5e6ba2aaf4f))
* add JSON tags and Dapr key schema for state store migration ([#258](https://github.com/nbyl/metio/issues/258)) ([6621c5a](https://github.com/nbyl/metio/commit/6621c5af17edc38fcadcbb2c74f753480e1b0a24))
* add postgres_mode variable and mount connection-string secret ([#325](https://github.com/nbyl/metio/issues/325)) ([c09390b](https://github.com/nbyl/metio/commit/c09390b4808659165726e3cb51a69e3ae3e2b521))
* add postgres_mode variable and mount connection-string secret ([#325](https://github.com/nbyl/metio/issues/325)) ([2a101a5](https://github.com/nbyl/metio/commit/2a101a56e4000fc28f3ca527f56ceb51fd2a8013))
* allow users to update machine agent from UI ([#309](https://github.com/nbyl/metio/issues/309)) ([a9676ae](https://github.com/nbyl/metio/commit/a9676ae69630922257c48b83a8abd820aedaed14))
* allow users to update machine agent from UI ([#309](https://github.com/nbyl/metio/issues/309)) ([912c8c2](https://github.com/nbyl/metio/commit/912c8c24fd7cfd3e8873436cc40558481732b051))
* enable Dapr runtime in local development environment ([d9f38c2](https://github.com/nbyl/metio/commit/d9f38c225c78c8545a46bb38327f27d25b9aaef2))
* enable Dapr runtime in local development environment ([#252](https://github.com/nbyl/metio/issues/252)) ([715f00a](https://github.com/nbyl/metio/commit/715f00a2610b37eb9542e72aa83b33cd97f9daa2))
* hide Players and Uptime stats when server is offline ([7a4441d](https://github.com/nbyl/metio/commit/7a4441d109e6659d129085fc2817fafaec820ee4))
* hide Players and Uptime stats when server is offline ([d197136](https://github.com/nbyl/metio/commit/d1971368fee57c347c2fd9782fa8c585960c2d38))
* implement DaprDB adapter with 24 DB interface methods ([01cc88d](https://github.com/nbyl/metio/commit/01cc88dfcac77ead79709e8661a77b07273b1bc0))
* implement DaprDB adapter with 24 DB interface methods ([#256](https://github.com/nbyl/metio/issues/256)) ([220281a](https://github.com/nbyl/metio/commit/220281a7418eb97a82324438371a116e3dc39e19))
* provision Cloud SQL instance in cloudsql mode ([#327](https://github.com/nbyl/metio/issues/327)) ([92a36ed](https://github.com/nbyl/metio/commit/92a36ed786442eb7d2f5555d149d2eea809d4d92))
* provision Cloud SQL instance in cloudsql mode ([#327](https://github.com/nbyl/metio/issues/327)) ([4cdb596](https://github.com/nbyl/metio/commit/4cdb59688cb53bfc7c1249cf32f10f046a04071b))
* remove Firestore resources and datastore permissions ([#328](https://github.com/nbyl/metio/issues/328)) ([9c55d3e](https://github.com/nbyl/metio/commit/9c55d3e97f7b9f4a8cff52062381ee1958e1c09a))
* remove Firestore resources and datastore permissions ([#328](https://github.com/nbyl/metio/issues/328)) ([b7666dc](https://github.com/nbyl/metio/commit/b7666dc39c01ee6c2457e02a23a6c96b99354ae7))
* replace Datastore emulator with local Postgres container in dev targets ([#326](https://github.com/nbyl/metio/issues/326)) ([eae841d](https://github.com/nbyl/metio/commit/eae841d5d532de98a02ccf076cdc4fa8290428c6))
* replace Datastore emulator with local Postgres container in dev targets ([#326](https://github.com/nbyl/metio/issues/326)) ([6faaa27](https://github.com/nbyl/metio/commit/6faaa27f03abe4eededa60f1940185071d64e1b0))
* run controller on Dapr statestore when DB_BACKEND=dapr ([888251e](https://github.com/nbyl/metio/commit/888251e6d3ad3a279b2e1af76c483100e1be400b))
* switch baked Dapr component to state.postgresql ([#324](https://github.com/nbyl/metio/issues/324)) ([ddb660d](https://github.com/nbyl/metio/commit/ddb660d0b5c02c51fdc9e52ac94dda24c7d0e68a))
* switch baked Dapr component to state.postgresql ([#324](https://github.com/nbyl/metio/issues/324)) ([1dd7950](https://github.com/nbyl/metio/commit/1dd7950d2699f531b19170c7a44264f9aebce437))
* **ui:** remove final-backup step from destroy wizard ([c94058e](https://github.com/nbyl/metio/commit/c94058ed6df11cdd83336da9c3e123efee57a459))
* **ui:** remove final-backup step from destroy wizard ([73a47a7](https://github.com/nbyl/metio/commit/73a47a703ebba9c27286827a3ed913bc560f440f)), closes [#345](https://github.com/nbyl/metio/issues/345)


### Bug Fixes

* add daprd startup probe, remove --app-port for Cloud Run depends_on ([af29ceb](https://github.com/nbyl/metio/commit/af29ceb5f3f5600d5f7f00a6fa70ac408433299d))
* add explicit command for daprd sidecar ([a874b1a](https://github.com/nbyl/metio/commit/a874b1a61dd83a9876a809b58f62162ff4319ebc))
* bump Go base image to 1.26 for Dapr SDK compatibility ([#256](https://github.com/nbyl/metio/issues/256)) ([05d3042](https://github.com/nbyl/metio/commit/05d30421d01f9afaf4969d499de4aa2510b64602))
* correct Dapr firestore component field names and daprd readiness check ([#252](https://github.com/nbyl/metio/issues/252)) ([78fea0c](https://github.com/nbyl/metio/commit/78fea0c1c230f772aefc68c4a31bb48374e26224))
* **deploy:** reference BYO connection string via external secret ([743bfae](https://github.com/nbyl/metio/commit/743bfae0ba0160b89e6e362adffe22500bc7e044))
* **deploy:** reference BYO connection string via external secret ([9908692](https://github.com/nbyl/metio/commit/9908692240d796ae92972cff6e2ef42a69e95048))
* **deploy:** resolve Cloud SQL cloudsql-mode deployment failures ([7e12513](https://github.com/nbyl/metio/commit/7e125138c127d35a43f223dc988ad0185fd6e407))
* **deploy:** resolve Cloud SQL cloudsql-mode deployment failures ([0cec207](https://github.com/nbyl/metio/commit/0cec2070ae380619e9a371ea532b8be113a9445e))
* derive machine-agent registry host from image URL to fix cross-region pulls ([4e3cd21](https://github.com/nbyl/metio/commit/4e3cd21a15c7c3a701ee7e0dc402b5d65f55f1b3))
* expose controller ingress port 8080 for Cloud Run multi-container ([6f23aa8](https://github.com/nbyl/metio/commit/6f23aa8fa3b4bea0a85c6cf8a14fd8e0e4f89bb5))
* remove broken -target from deploy-controller Makefile ([ef0fd84](https://github.com/nbyl/metio/commit/ef0fd84d0a5a99c3f2bb199397c82e73ad0b2878))
* resolve Dapr Firestore project id via ADC auto-detection ([1912294](https://github.com/nbyl/metio/commit/1912294821ff3b21698f981f424f753d87531b06))
* suppress tofu warnings by using secret_data_wo write-only alternative ([6fbc550](https://github.com/nbyl/metio/commit/6fbc550a92eb962a1c1ec08a4a25d518664f2bca))
* treat PlayerDB 400 as not-found and fall back on Mojang errors ([#332](https://github.com/nbyl/metio/issues/332)) ([4e62ed8](https://github.com/nbyl/metio/commit/4e62ed83108a907d1c097766e17418df37af540e))
* treat PlayerDB 400 as not-found and fall back on Mojang errors ([#332](https://github.com/nbyl/metio/issues/332)) ([e544028](https://github.com/nbyl/metio/commit/e544028069ad4a3fabe2e0bbd476f179a65afb1d))
* **web:** support eslint 10 in CI build ([4315c53](https://github.com/nbyl/metio/commit/4315c536aa6008407bdf9d96e05ea497adbcf74b))
* whitelist endpoint returns 500 for new servers ([2eb9662](https://github.com/nbyl/metio/commit/2eb96624541b3629dcca3ff3b2050e916af9221c))
* whitelist endpoint returns 500 for new servers ([897a8b5](https://github.com/nbyl/metio/commit/897a8b54cac3fad64d3fa4838d0121db9e0eb9d9)), closes [#344](https://github.com/nbyl/metio/issues/344)
* wire Dapr emulator env into daprd, unify develop target ([#252](https://github.com/nbyl/metio/issues/252)) ([5c647e6](https://github.com/nbyl/metio/commit/5c647e659cc9b6ec85e6ec9d4a2edf0bd488a58c))

## [1.5.0](https://github.com/nbyl/metio/compare/v1.4.1...v1.5.0) (2026-07-07)


### Features

* implement controller agent API with HMAC-JWT per-server token auth ([#300](https://github.com/nbyl/metio/issues/300)) ([6d4da14](https://github.com/nbyl/metio/commit/6d4da146b8c9142c8c3e15222db6031cf89e5326))
* implement controller agent API with HMAC-JWT per-server token auth ([#300](https://github.com/nbyl/metio/issues/300)) ([7b02679](https://github.com/nbyl/metio/commit/7b02679b95a3a080769334b0418001cede302e2a))


### Bug Fixes

* save image tags to build/*.txt and push to Artifact Registry in local docker targets ([391775d](https://github.com/nbyl/metio/commit/391775d3c504513e512b737258ca9db2a79fbeb5))
* split docker image targets for CI and deploy, run image builds on PRs ([250a4d9](https://github.com/nbyl/metio/commit/250a4d93c54d8f29a08446024658baf98e7a8991))
* split docker image targets for CI and deploy, run image builds on PRs ([2280df1](https://github.com/nbyl/metio/commit/2280df1625b703925ab47efed1df2fecce9d3a5e))

## [1.4.1](https://github.com/nbyl/metio/compare/v1.4.0...v1.4.1) (2026-07-02)


### Bug Fixes

* pass ExistingAddress through Cloud Tasks handler ([2d43d51](https://github.com/nbyl/metio/commit/2d43d51c7f75f23b2b7e6ed1e97241d7eff5e656))
* pass ExistingAddress through Cloud Tasks handler ([68e7d47](https://github.com/nbyl/metio/commit/68e7d477e190c946c778f3959b943799472723c6))

## [1.4.0](https://github.com/nbyl/metio/compare/v1.3.0...v1.4.0) (2026-07-02)


### Features

* support importing existing static IP addresses when provisioning servers ([7dd8dde](https://github.com/nbyl/metio/commit/7dd8dde832f09de29e554a2455a50634fc8c11d4))
* support importing existing static IP addresses when provisioning servers ([4e7b477](https://github.com/nbyl/metio/commit/4e7b47749f22b2b5577d40302bdf78a25156e0e2)), closes [#293](https://github.com/nbyl/metio/issues/293)


### Bug Fixes

* refresh stack after import and enable code generation for pulumi import ([e87e018](https://github.com/nbyl/metio/commit/e87e0189cba139b97584eae2e3c328da78fa90df))

## [1.3.0](https://github.com/nbyl/metio/compare/v1.2.0...v1.3.0) (2026-06-30)


### Features

* replace go-semantic-release with release-please ([ecc18e2](https://github.com/nbyl/metio/commit/ecc18e29b335729339b99777297ce8b7be04bc66))
* replace go-semantic-release with release-please ([e98dc9c](https://github.com/nbyl/metio/commit/e98dc9cd695e8cbabcc1ad824534cbaac27664fa))


### Bug Fixes

* add environment: main to release job ([7947e55](https://github.com/nbyl/metio/commit/7947e558482ea9097acf3db93e30c9319fba66c6))
* add environment: main to release job ([3577d26](https://github.com/nbyl/metio/commit/3577d263239b72f3d8f35e707b9935b9c0c103a9))
* add release-please extra-files for variables.tf ([5d8f990](https://github.com/nbyl/metio/commit/5d8f990bbcc9e49f3929c5e03c1b99ef5fb41e0c))
* add release-please extra-files for variables.tf ([a06d54a](https://github.com/nbyl/metio/commit/a06d54a93153bb60f4141328f015ea0f1230e95a))
* rename release-please config and add checkout step ([9f5d3bd](https://github.com/nbyl/metio/commit/9f5d3bd2b0cb50b6b51471f66b4d075cf59b85b7))
* rename release-please config file and add checkout step ([9672939](https://github.com/nbyl/metio/commit/9672939ed1a1a1d3b68c5fd8d9fde142f24f3522))
* use RELEASE_PLEASE_TOKEN secret for release-please-action ([0d05bd0](https://github.com/nbyl/metio/commit/0d05bd04843938c8a33cf90676b33841e3aa3977))
* use RELEASE_PLEASE_TOKEN secret for release-please-action ([e6fe4f3](https://github.com/nbyl/metio/commit/e6fe4f3e750c2215baf47e26a5921e74c10f85d3))
