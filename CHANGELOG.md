# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Changed

- The user in context is now more enriched (with fields like job, rank, current unit...)
- The metrics metric now contains a timestamp and more information about the user.
- Add a mime type update option.
- Add swagger documentation 

## [v2.3.0] - 2020-11-23

## Added

- FEAT([185](https://github.com/meateam/api-gateway/pull/185)): Swagger documentation 
  - Now you can go to /api/docs and see the documentation 

## [v2.2.1] - 2020-11-23

## Added
 
- FEAT([167](https://github.com/meateam/api-gateway/pull/167)): add a mime type update option.

## [v2.2.0] - 2020-11-18

## Added

- FEAT([89](https://github.com/meateam/authentication-service/pull/89)): add curl on docker image

## [v2.1.1] - 2020-11-11

### Fixed

- BUG([190](https://github.com/meateam/api-gateway/pull/190)): Dropbox Auth-Type fix.

## [v2.1.0] - 2020-11-1

### Added

- Files will now have an `appID` field with the value of the client ID which created the file.

- External apps (which does not include `Dropbox` and `Docs`) can now preform actions on behalf of users by authenticating with `authcode` grant-type (oauth):
  - The actions will be granted by scopes and may include: `upload`, `download`, `get-metadata`, `share`, `delete`.
  - New files will be created with the new `appID` field of the external app.
  - All of the actions, except `upload` will only be permitted on fiels with corresponding `appID` (the ID of the external app).
  - The new created files will not be visible in the main client's pages, but only by search. 
 

### Changed

- Authentication for Dropbox will **only** work with the `Auth-Type` header value of `Dropbox` instead of `Service`.


## [v2.0.0] - 2020-10-28

### Added

- FEAT([153](https://github.com/meateam/api-gateway/pull/153)): add update feature

- FEAT([162](https://github.com/meateam/api-gateway/pull/162)): add auth startegy for docs

[unreleased]: https://github.com/meateam/api-gateway/compare/master...develop
[v2.3.0]: https://github.com/meateam/api-gateway/compare/v2.2.1...v2.3.0
[v2.2.1]: https://github.com/meateam/api-gateway/compare/v2.2.0...v2.2.1
[v2.2.0]: https://github.com/meateam/api-gateway/compare/v2.1.1...v2.2.0
[v2.1.1]: https://github.com/meateam/api-gateway/compare/v2.1.0...v2.1.1
[v2.1.0]: https://github.com/meateam/api-gateway/compare/v2.0.0...v2.1.0
[v2.0.0]: https://github.com/meateam/api-gateway/compare/v1.3...v2.0.0
