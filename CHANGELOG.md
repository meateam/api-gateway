# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## [v4.4.1] - 2021-05-30


### Fixed

- hotfix([237](https://github.com/meateam/api-gateway/pull/237)): add pending status for transfer
- hotfix([236](https://github.com/meateam/api-gateway/pull/236)): fix delete file in shared folder & fix transfer info premission
- hotfix([201](https://github.com/meateam/api-gateway/issues/201)): changed the delete order between the file and its permissions and added a rollback for the permissions

## [v4.4.0] - 2021-05-18

- minor([185](https://github.com/meateam/api-gateway/pull/185)): Added CTS upn transfer -fix

## [v4.3.0] - 2021-05-16

### Fixed

- minor: ([234](https://github.com/meateam/api-gateway/pull/234)): Added CTS upn transfer

## [v4.2.2] - 2021-05-14

### Fixed

- hotfix: ([231](https://github.com/meateam/api-gateway/pull/231)): allow cargo->drive transfer with identical `from` and `to`

## [v4.2.1] - 2021-04-27

### Added

- hotfix: ([229](https://github.com/meateam/api-gateway/pull/229)): add env variables

## [v4.1.0] - 2021-04-11

### Added

- minor: ([218](https://github.com/meateam/api-gateway/pull/218)): add advance search by mail/T.

## [v4.0.0] - 2021-03-25

### Added

- major:([215](https://github.com/meateam/api-gateway/issues/215)): add cts integration

- minor:([221](https://github.com/meateam/api-gateway/pull/221)): added url for local office

## [v3.0.1] - 2021-02-28

### Added

- hotfix:([216](https://github.com/meateam/api-gateway/pull/216)): added new env:
  1. GW_BAM_SUPPORT_NUMBER
  2. GW_APPROVAL_UI_URL

## [v3.0.0] - 2021-01-13

### Added

- major:([199](https://github.com/meateam/api-gateway/pull/199)): added pagination for shared files.

### Fixed

- hotfix:(BUG)([208](https://github.com/meateam/api-gateway/pull/208)): fix connection pool pointer bug.

## [v2.3.0] - 2020-12-24

### Changed

- The user in context is now more enriched (with fields like job, rank, current unit...)
- The metrics metric now contains a timestamp and more information about the user.

### Added

- FEAT([185](https://github.com/meateam/api-gateway/pull/185)): Swagger documentation go to /api/docs
- FEAT([167](https://github.com/meateam/api-gateway/pull/167)): add a mime type update option.
- FEAT([195](https://github.com/meateam/api-gateway/issues/195)): call with grpc to user service method to get if user can approve
- FEAT: add new envs: bam_support_number, bereshit_support_link for approver support

## [v2.2.0] - 2020-11-18

### Added

- FEAT([89](https://github.com/meateam/authentication-service/pull/89)): add curl on docker image

## [v2.1.1] - 2020-11-11

### Fixed

- BUG([190](https://github.com/meateam/api-gateway/pull/190)): Dropbox Auth-Type fix.

## [v2.1.0] - 2020-11-1

### Added

- FEAT([143](https://github.com/meateam/api-gateway/pull/143)): Added + Changed

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
[v4.4.0]: https://github.com/meateam/api-gateway/compare/v4.3.0...v4.4.0
[v4.3.0]: https://github.com/meateam/api-gateway/compare/v4.2.2...v4.3.0
[v4.2.2]: https://github.com/meateam/api-gateway/compare/v4.2.1...v4.2.2
[v4.2.1]: https://github.com/meateam/api-gateway/compare/v4.1.0...v4.2.1
[v4.1.0]: https://github.com/meateam/api-gateway/compare/v4.0.0...v4.1.0
[v4.0.0]: https://github.com/meateam/api-gateway/compare/v3.0.1...v4.0.0
[v3.0.1]: https://github.com/meateam/api-gateway/compare/v3.0.0...v3.0.1
[v3.0.0]: https://github.com/meateam/api-gateway/compare/v2.3.0...v3.0.0
[v2.3.0]: https://github.com/meateam/api-gateway/compare/v2.2.0...v2.3.0
[v2.2.0]: https://github.com/meateam/api-gateway/compare/v2.1.1...v2.2.0
[v2.1.1]: https://github.com/meateam/api-gateway/compare/v2.1.0...v2.1.1
[v2.1.0]: https://github.com/meateam/api-gateway/compare/v2.0.0...v2.1.0
[v2.0.0]: https://github.com/meateam/api-gateway/compare/v1.3...v2.0.0
