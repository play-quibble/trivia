# [1.5.0](https://github.com/play-quibble/trivia/compare/v1.4.0...v1.5.0) (2026-04-24)


### Features

* **web:** add useGameSocket core hook ([eb6ed4b](https://github.com/play-quibble/trivia/commit/eb6ed4bad7c9fe17ebf2fd6f050421b41d8d6a5b))

# [1.4.0](https://github.com/play-quibble/trivia/compare/v1.3.0...v1.4.0) (2026-04-21)


### Features

* **realtime:** send full player roster on lobby connect and player join ([066aaf7](https://github.com/play-quibble/trivia/commit/066aaf79cef6f23ce0ed4cb4a7f876514a530394))

# [1.3.0](https://github.com/benbotsford/trivia/compare/v1.2.0...v1.3.0) (2026-04-21)


### Features

* add Helm chart and chart-release workflow ([b64a130](https://github.com/benbotsford/trivia/commit/b64a1300330719464a77e4fb26919085d97532e5))

# [1.2.0](https://github.com/benbotsford/trivia/compare/v1.1.2...v1.2.0) (2026-04-21)


### Features

* Remove migrate service from docker-compose and enable auto-migration in the API service ([383325b](https://github.com/benbotsford/trivia/commit/383325b142e4dc2e42a7e4f3b3ab5b4245805cfa))

## [1.1.2](https://github.com/benbotsford/trivia/compare/v1.1.1...v1.1.2) (2026-04-21)


### Bug Fixes

* expand docker build matrix across all platforms ([538fe2a](https://github.com/benbotsford/trivia/commit/538fe2a693d0a04675ff3c966e6eda6b9b4dcc63))

## [1.1.1](https://github.com/benbotsford/trivia/compare/v1.1.0...v1.1.1) (2026-04-21)


### Bug Fixes

* trigger release to validate multi-arch build matrix ([86f8857](https://github.com/benbotsford/trivia/commit/86f8857689d86b29554e88dc2ff570f32428f417))

# [1.1.0](https://github.com/benbotsford/trivia/compare/v1.0.4...v1.1.0) (2026-04-21)


### Features

* add docker-compose for local stack testing; build multi-platform images ([20d675a](https://github.com/benbotsford/trivia/commit/20d675a1408edbc5a7e7190c10c28833d2edd2a5))

## [1.0.4](https://github.com/benbotsford/trivia/compare/v1.0.3...v1.0.4) (2026-04-21)


### Bug Fixes

* remove public/ copy from web Dockerfile (directory does not exist) ([b16dd67](https://github.com/benbotsford/trivia/commit/b16dd67ed636c5e92b4d6b687243953bd8017edf))

## [1.0.3](https://github.com/benbotsford/trivia/compare/v1.0.2...v1.0.3) (2026-04-21)


### Bug Fixes

* restore tsconfig.json in web Docker build context ([7dfb5b1](https://github.com/benbotsford/trivia/commit/7dfb5b1a16ce4c962512c84eb4e177305759c8b6))

## [1.0.2](https://github.com/benbotsford/trivia/compare/v1.0.1...v1.0.2) (2026-04-21)


### Bug Fixes

* add explicit Quiz[] type annotation to quizzes page ([cfffdb7](https://github.com/benbotsford/trivia/commit/cfffdb75045ae8dc422f05f4fef514a076c6d8bd))

## [1.0.1](https://github.com/benbotsford/trivia/compare/v1.0.0...v1.0.1) (2026-04-21)


### Bug Fixes

* resolve CI lint failures in API and web ([26e73b8](https://github.com/benbotsford/trivia/commit/26e73b812630be12df2438e4cbb0d5a260aff2fc))

# 1.0.0 (2026-04-20)


### Features

* add Next.js frontend with question bank UI and dev API wiring ([74a62eb](https://github.com/benbotsford/trivia/commit/74a62ebf5490e2815d3238fb284c37bf139da541)), closes [#00338](https://github.com/benbotsford/trivia/issues/00338) [#C60C30](https://github.com/benbotsford/trivia/issues/C60C30)
* **api:** implement question bank CRUD endpoints ([b56fca2](https://github.com/benbotsford/trivia/commit/b56fca25ba2088af1fe21240af2975240c4447bc))
