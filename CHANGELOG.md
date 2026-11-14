# Changelog

All notable changes to this project will be documented in this file.

## [0.2.0](https://github.com/baptistegh/terraform-provider-polaris/compare/v0.1.1...v0.2.0) (2026-11-14)


### Features

* add catalog, catalog role, principal role and list data sources ([#18](https://github.com/baptistegh/terraform-provider-polaris/issues/18)) ([380d554](https://github.com/baptistegh/terraform-provider-polaris/commit/380d55439f07a80c37a1f431281ba21f142052cc)), closes [#14](https://github.com/baptistegh/terraform-provider-polaris/issues/14)
* add polaris_catalog and polaris_semantic_model_grant resources ([#17](https://github.com/baptistegh/terraform-provider-polaris/issues/17)) ([a3ea525](https://github.com/baptistegh/terraform-provider-polaris/commit/a3ea5250f9dcc137b04003149b423dff25838be6)), closes [#13](https://github.com/baptistegh/terraform-provider-polaris/issues/13)
* add polaris_namespace resource and data sources ([#28](https://github.com/baptistegh/terraform-provider-polaris/issues/28)) ([9e1f194](https://github.com/baptistegh/terraform-provider-polaris/commit/9e1f194d27a74893decc7afc26041f8e073b43c2))
* add polaris_policy and polaris_policy_attachment resources ([#29](https://github.com/baptistegh/terraform-provider-polaris/issues/29)) ([9a7aae6](https://github.com/baptistegh/terraform-provider-polaris/commit/9a7aae663b8d70fa4b72bffaa67100d31939eaa3)), closes [#24](https://github.com/baptistegh/terraform-provider-polaris/issues/24)
* add polaris_principal_credentials resource ([#19](https://github.com/baptistegh/terraform-provider-polaris/issues/19)) ([2cb3cbf](https://github.com/baptistegh/terraform-provider-polaris/commit/2cb3cbf938af41aa028104035e7df702016b7209)), closes [#15](https://github.com/baptistegh/terraform-provider-polaris/issues/15)
* add polaris_semantic_model resource and data sources ([#31](https://github.com/baptistegh/terraform-provider-polaris/issues/31)) ([0242173](https://github.com/baptistegh/terraform-provider-polaris/commit/0242173972508e7f32885b50731e80b25aa3c358)), closes [#27](https://github.com/baptistegh/terraform-provider-polaris/issues/27)


### Bug Fixes

* **deps:** update module github.com/oapi-codegen/runtime to v1.8.0 ([262cfb7](https://github.com/baptistegh/terraform-provider-polaris/commit/262cfb736fb9eb21e13ebd4ffa95dd6e89609466))


### Documentation

* add agent skills for new types, commits and spec coverage ([#16](https://github.com/baptistegh/terraform-provider-polaris/issues/16)) ([8d0666f](https://github.com/baptistegh/terraform-provider-polaris/commit/8d0666f0a4bca19e764e54d65e1cf74f2dc252cb))
* forbid AI co-author credit, require Assisted-By/Generated-By ([#11](https://github.com/baptistegh/terraform-provider-polaris/issues/11)) ([03b41c6](https://github.com/baptistegh/terraform-provider-polaris/commit/03b41c6bced6803d9f706e63eb55d6046fe53c6e))
* keep the AI attribution trailer to commit messages ([#25](https://github.com/baptistegh/terraform-provider-polaris/issues/25)) ([468e501](https://github.com/baptistegh/terraform-provider-polaris/commit/468e50132a5b7b23cbf2ad081ecc55bcbe021f28))

## [0.1.1] - 2026-06-21

### Miscellaneous

- **ci**: Resolve CHANGELOG use by goreleaser

## [0.1.0] - 2026-06-21

### Miscellaneous

- Init repository
- **ci**: Set up CI on main and release on tags
- Add renovate configuration and oapi-codegen as a go.mod tool
- Apply lint fixes
- **deps**: Update github actions ([#1](https://github.com/baptistegh/terraform-provider-polaris/issues/1))
