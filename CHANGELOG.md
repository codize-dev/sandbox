# Changelog

## [0.7.3](https://github.com/codize-dev/sandbox/compare/v0.7.2...v0.7.3) (2026-04-17)


### Bug Fixes

* **deps:** update dependency ruby to v3.4.9 ([#28](https://github.com/codize-dev/sandbox/issues/28)) ([d49e4ac](https://github.com/codize-dev/sandbox/commit/d49e4ac12e253f5d06538e9928b5a657374aca28))
* **deps:** update go to v1.26.2 ([#34](https://github.com/codize-dev/sandbox/issues/34)) ([b64eca9](https://github.com/codize-dev/sandbox/commit/b64eca980c64bf2cf1cf18e6f711cbaac50e70a0))

## [0.7.2](https://github.com/codize-dev/sandbox/compare/v0.7.1...v0.7.2) (2026-04-15)


### Bug Fixes

* **deps:** update dependency jdx/mise to v2026.3.16 ([#30](https://github.com/codize-dev/sandbox/issues/30)) ([e91347b](https://github.com/codize-dev/sandbox/commit/e91347bf34330f739b00f82fdcb7a8ba64f49409))
* enable Renovate auto-updates for go directive in go.mod ([dee956d](https://github.com/codize-dev/sandbox/commit/dee956d6b91f4c5c8cb49c698fbb9bd413282ef5))
* exclude DOM types from node-typescript tsconfig to prevent global name conflicts ([badca20](https://github.com/codize-dev/sandbox/commit/badca2039b1f65d8bdb0893b0b1aeee9d98a2eb4))
* extend Renovate gomod manager to cover go.mod.tmpl ([aaf706b](https://github.com/codize-dev/sandbox/commit/aaf706bbbc0d83ed90b451be4ef8ede09a90c2f3))
* update mise to v2026.4.11 to fix Python freethreaded build selection bug ([4ebd58f](https://github.com/codize-dev/sandbox/commit/4ebd58f9166b1378c13b75f0b5f1f87c535b31c8))
* use current symlink paths in security E2E tests instead of versioned paths ([5b4fd77](https://github.com/codize-dev/sandbox/commit/5b4fd77946365d166b2de8e57832ea420dc434ef))
* use full semver in go.mod.tmpl for Renovate compatibility ([d171f25](https://github.com/codize-dev/sandbox/commit/d171f25f7ba87673dd1a688e30b1e0ee72468e9f))
* use version-agnostic symlinks for runtime paths ([3987680](https://github.com/codize-dev/sandbox/commit/398768085455c22f29ca2a9ffd83c0137aa21640))

## [0.7.1](https://github.com/codize-dev/sandbox/compare/v0.7.0...v0.7.1) (2026-03-10)


### Bug Fixes

* add --disable-wasm-trap-handler to Node.js runtimes to prevent OOM ([7b51661](https://github.com/codize-dev/sandbox/commit/7b516619a6cb1e8741d32fba9cfee2ed75896471))
* enable Renovate auto-updates for mise-managed runtime versions ([34acc0e](https://github.com/codize-dev/sandbox/commit/34acc0ea299b187797e57701bfdc005f55df9875))

## [0.7.0](https://github.com/codize-dev/sandbox/compare/v0.6.0...v0.7.0) (2026-03-09)


### Features

* add --metrics flag to toggle /metrics endpoint ([6e5fadd](https://github.com/codize-dev/sandbox/commit/6e5fadd98124fa328fad6fa10788e860aec29b1d))
* set sandbox HOME directory to /sandbox instead of /tmp ([1cd4afd](https://github.com/codize-dev/sandbox/commit/1cd4afdbb10db7e3903fe2cc7604bb1fd8b98be9))

## [0.6.0](https://github.com/codize-dev/sandbox/compare/v0.5.0...v0.6.0) (2026-03-09)


### Features

* add base64_encoded field to make base64 input opt-in ([de14dc5](https://github.com/codize-dev/sandbox/commit/de14dc59ea7e1313e55ad0358f3a88794f5a95a5))


### Bug Fixes

* Update go 1.25 to 1.26 ([6d1f988](https://github.com/codize-dev/sandbox/commit/6d1f988d7290b8949028d0e30096457f40ce6676))

## [0.5.0](https://github.com/codize-dev/sandbox/compare/v0.4.4...v0.5.0) (2026-03-09)


### Features

* add Prometheus /metrics endpoint for concurrency and queue gauges ([a869ca7](https://github.com/codize-dev/sandbox/commit/a869ca7939a7bd3187b58959698ea6d6d15d46d2))

## [0.4.4](https://github.com/codize-dev/sandbox/compare/v0.4.3...v0.4.4) (2026-03-09)


### Bug Fixes

* improve go fork bomb test to verify process limit enforcement ([e25544e](https://github.com/codize-dev/sandbox/commit/e25544e492b99761a09f1cc0b9ca688b332c9533))

## [0.4.3](https://github.com/codize-dev/sandbox/compare/v0.4.2...v0.4.3) (2026-03-08)


### Bug Fixes

* update nsjail base image to commit 222f2fa ([6480042](https://github.com/codize-dev/sandbox/commit/648004251dd952545bd7459ee2f57f85243651bb))

## [0.4.2](https://github.com/codize-dev/sandbox/compare/v0.4.1...v0.4.2) (2026-03-08)


### Bug Fixes

* correct nsjail image digest to manifest list hash ([101a839](https://github.com/codize-dev/sandbox/commit/101a839b60647337c87d16c070da1b26b0d552aa))

## [0.4.1](https://github.com/codize-dev/sandbox/compare/v0.4.0...v0.4.1) (2026-03-08)


### Bug Fixes

* **deps:** update alpine docker tag to v3.23.3 ([#7](https://github.com/codize-dev/sandbox/issues/7)) ([507d168](https://github.com/codize-dev/sandbox/commit/507d16885b523a8023efcc77926cfc4f679265ce))

## [0.4.0](https://github.com/codize-dev/sandbox/compare/v0.3.0...v0.4.0) (2026-03-08)


### Features

* add /healthz endpoint for liveness checks ([2f89b3e](https://github.com/codize-dev/sandbox/commit/2f89b3e47d74efb553e5d80bbad2405678d32dc8))
* add package-lock.json to node-typescript restricted files and use npm ci ([363b92f](https://github.com/codize-dev/sandbox/commit/363b92f91ed06a85039123d3d0ff2b6b3dffa9dd))

## [0.3.0](https://github.com/codize-dev/sandbox/compare/v0.2.0...v0.3.0) (2026-03-08)


### Features

* add Python 3.13.12 runtime support ([cbebf3e](https://github.com/codize-dev/sandbox/commit/cbebf3eb3b2c31e951a2e5bd20bb0f48396e5cf3))
* add Rust 1.94.0 runtime support ([a42f253](https://github.com/codize-dev/sandbox/commit/a42f2538cb177088595f628680ea445b43b9b65e))
* add TypeScript (node-typescript) runtime support ([9af8e49](https://github.com/codize-dev/sandbox/commit/9af8e493957a80c4f1c3925958a88d0c2e6ce105))

## [0.2.0](https://github.com/codize-dev/sandbox/compare/v0.1.1...v0.2.0) (2026-03-07)


### Features

* add concurrency limiter middleware with queue management ([73e225d](https://github.com/codize-dev/sandbox/commit/73e225dea9446dbff35f3e05c0488b87dfcc30c3))
* add panic recover middleware to prevent server crashes ([fccee12](https://github.com/codize-dev/sandbox/commit/fccee1257a93309b8533798e017f96f52f713508))


### Bug Fixes

* add IdleTimeout to prevent idle keep-alive connections from lingering indefinitely ([2e099d2](https://github.com/codize-dev/sandbox/commit/2e099d2a9b6567da0e8c8bc9ca88ff4a52b0eaa0))

## [0.1.1](https://github.com/codize-dev/sandbox/compare/v0.1.0...v0.1.1) (2026-03-06)


### Bug Fixes

* override Echo v5 default WriteTimeout to prevent long-running sandbox responses from being dropped ([1668d13](https://github.com/codize-dev/sandbox/commit/1668d1384c3b80a8cc18d96a6318a2f4acb3ac5f))

## [0.1.0](https://github.com/codize-dev/sandbox/compare/v0.0.1...v0.1.0) (2026-03-06)


### Features

* add structured logging with slog and fix silent error handling ([5d5ddb3](https://github.com/codize-dev/sandbox/commit/5d5ddb3f4e1623e35c300d3f20a4a1a47ce178b6))


### Bug Fixes

* modernize ([bb0a299](https://github.com/codize-dev/sandbox/commit/bb0a299efd3d80a65d02afea5ea7929dded08418))
* remove hardcoded serve subcommand from ENTRYPOINT ([bfd4a32](https://github.com/codize-dev/sandbox/commit/bfd4a322d38606ba05b98c3bf29bbd4713b46656))
* trigger version bump on Dockerfile dependency updates via Renovate ([8de6a26](https://github.com/codize-dev/sandbox/commit/8de6a267bf3038404dd8675775128b9150d1c734))
* use 137 exit code for output limit exceeded instead of -1 ([26aee35](https://github.com/codize-dev/sandbox/commit/26aee35fbef4ed85116b4db0a4f0a201c060eb54))

## [0.0.1](https://github.com/codize-dev/sandbox/compare/v0.0.0...v0.0.1) (2026-03-06)


### Features

* Release v0.0.1 ([6d8f7bd](https://github.com/codize-dev/sandbox/commit/6d8f7bdda02f764fd2b0d84af21fc36fbbc09572))

## 0.0.0 (2026-03-06)


### Features

* add --max-body-size flag to limit HTTP request body size ([304f580](https://github.com/codize-dev/sandbox/commit/304f580b66990e89771c26059b9d4c541ed83cdb))
* add --max-file-size flag to limit individual file size per request ([345f0d7](https://github.com/codize-dev/sandbox/commit/345f0d7ba0db3b54839d44373bf91713e9f17f7a))
* add --max-files flag to limit the number of files per request ([a2cc8d2](https://github.com/codize-dev/sandbox/commit/a2cc8d2f9866027dff3deee10749cd2ccd736a84))
* add /bin to PATH for all runtimes to match user expectations ([79b4ed1](https://github.com/codize-dev/sandbox/commit/79b4ed178e2fd4001d423dbb6af9711ae7994023))
* add /usr/bin to PATH and /bin symlink for command accessibility ([a9d5e6e](https://github.com/codize-dev/sandbox/commit/a9d5e6e1091127007b241093107c9b46ec6847db))
* add 255-byte file name length validation ([1cb7f9b](https://github.com/codize-dev/sandbox/commit/1cb7f9b33753d705f6336db8661ec9ef8ffe3055))
* add arch field to E2E framework and split architecture-dependent tests ([0cedefb](https://github.com/codize-dev/sandbox/commit/0cedefba8708003ff3fea28da034f76f83f502c4))
* add bash runtime support for shell script execution ([0d88644](https://github.com/codize-dev/sandbox/commit/0d88644a5e4ae57aa1920e9302156ce921c9e608))
* add basic Echo v5 HTTP server ([6802819](https://github.com/codize-dev/sandbox/commit/6802819967536f85dcf354d698f35e7812affee3))
* add cgroup CPU throttle to limit sandbox CPU usage per core ([04162fb](https://github.com/codize-dev/sandbox/commit/04162fb82610899ce8d34ff5d0323003e814b00c))
* add cgroup memory limit and swap restriction for sandbox OOM protection ([68b0075](https://github.com/codize-dev/sandbox/commit/68b0075ad1cf8f08aeecc5f54e7813249c2b25f3))
* add cgroup pids limit and separate Rlimits from Cgroups for type safety ([4364238](https://github.com/codize-dev/sandbox/commit/4364238e5a930046714ec323f52aea93654e099e))
* add Docker Compose configuration with privileged mode ([61600e9](https://github.com/codize-dev/sandbox/commit/61600e929a0e7177e4422beeb3e268bf76578ec8))
* add Go runtime support with compile-then-run execution model ([6adfca8](https://github.com/codize-dev/sandbox/commit/6adfca87e32f04274f3327d77c3abcc282e19dda))
* add GOCACHEPROG read-only cache helper for Go sandbox compilation ([fdc20bc](https://github.com/codize-dev/sandbox/commit/fdc20bc74c3a5e07360b6f7c51344dd665621469))
* add mise to runtime image via musl static binary ([07de470](https://github.com/codize-dev/sandbox/commit/07de47007936b24e51989de228bfd64baafc8f1f))
* add multi-stage Dockerfile with nsjail runtime ([4f57bba](https://github.com/codize-dev/sandbox/commit/4f57bba0aec1b877a5c50eb9cdff2332842f6a37))
* add nosuid and nodev mount flags to /tmp tmpfs via protobuf config ([a7d1633](https://github.com/codize-dev/sandbox/commit/a7d163383dffbeed0b21783e76ce2994a7824332))
* add nsjail --detect_cgroupv2 for cgroup v2 auto-detection ([ce815ce](https://github.com/codize-dev/sandbox/commit/ce815cec0ff5aba734a42c7d0e179e90f770587f))
* add nsjail --rlimit_cpu to limit per-process CPU time ([a1f3496](https://github.com/codize-dev/sandbox/commit/a1f34965ca58e740d1dbf6f9fd853ce7e73bb5e5))
* add nsjail rlimit hardening for memlock, rtprio, msgqueue, nproc, and stack ([0e82ef2](https://github.com/codize-dev/sandbox/commit/0e82ef206f3f3688adf25b47019a1975fc31bacf))
* add path traversal protection with file name validation and e2e tests ([d5f9c02](https://github.com/codize-dev/sandbox/commit/d5f9c024446077336200b38f35846028facd6452))
* add pre-installed golang.org/x/text package for Go sandbox ([9b7157f](https://github.com/codize-dev/sandbox/commit/9b7157ff0f952b29669b32b50c79084dea793879))
* add requests array and fill file type to E2E test framework ([2232015](https://github.com/codize-dev/sandbox/commit/223201568404b1428833c49527d78d479bea0a91))
* add Ruby runtime support to /v1/run endpoint ([d6e524d](https://github.com/codize-dev/sandbox/commit/d6e524d865a510dfb6fac6a3ff496269262e2b68))
* add seccomp-bpf syscall filtering policy for sandbox hardening ([b5c488a](https://github.com/codize-dev/sandbox/commit/b5c488a28a241755d821a7f8eb1417701df13ede))
* add signal field to API response for detecting signal-terminated processes ([10503a1](https://github.com/codize-dev/sandbox/commit/10503a1d7bc962cc0baa79a7215e699f1625855d))
* add YAML-driven E2E test framework with build tag isolation ([f4b4b27](https://github.com/codize-dev/sandbox/commit/f4b4b2745bb773868e63592229afe0e735622f0f))
* detect nsjail timeout via log pipe and add status field to response ([f13d16e](https://github.com/codize-dev/sandbox/commit/f13d16e2bec7a68eb775b1516a8502783a1ccf21))
* disable loopback interface inside sandbox via iface_no_lo ([41aea7f](https://github.com/codize-dev/sandbox/commit/41aea7f489db79a1c356d0cca430870b2849d443))
* enforce 1 MiB output limit and kill sandbox process on excess ([afc51b2](https://github.com/codize-dev/sandbox/commit/afc51b269b1f254ea33ae331e37199914fef7bd4))
* explicitly set clone_newnet in nsjail config for clarity ([fd9291e](https://github.com/codize-dev/sandbox/commit/fd9291e7251a8a98071721680847bc5d9087f822))
* install ca-certificates and gpg in runtime image ([16045f5](https://github.com/codize-dev/sandbox/commit/16045f5163166665634d022f257c77fa7ea4d641))
* install curl, wget, and mawk in sandbox environment ([af93855](https://github.com/codize-dev/sandbox/commit/af93855d96a1da190a23fbac871b0823ecb4d1cc))
* make execution timeout configurable via SANDBOX_RUN_TIMEOUT env var ([2a374da](https://github.com/codize-dev/sandbox/commit/2a374dabb699fe603285586fb0c9b2bac3206721))
* map sandbox UID/GID to nobody (65534) for non-root process isolation ([02d5b3d](https://github.com/codize-dev/sandbox/commit/02d5b3d49e6abbdc029de392a77238dc367adf9e))
* preinstall Node.js 24 via mise and add gpg-agent ([91b8524](https://github.com/codize-dev/sandbox/commit/91b8524300b5e3eab108261199a27481aa5fc921))
* reject user-submitted restricted files per runtime (go.mod, go.sum) ([ccd2684](https://github.com/codize-dev/sandbox/commit/ccd26840b6d5e38a8f895656385edba63f96840a))
* Release v0.0.0 ([9616bfd](https://github.com/codize-dev/sandbox/commit/9616bfda97032c588acf0ea128fb6d7dc76a52d1))
* replace --addr flag with --port and support PORT env var ([75e43c6](https://github.com/codize-dev/sandbox/commit/75e43c6e5cab7209ba91de0480cf1ce77df655b4))
* replace /tmp host bind mount with in-sandbox tmpfs (64 MiB) ([f4fd905](https://github.com/codize-dev/sandbox/commit/f4fd905a4e8d9563cda60bdc4a07bfa28fce0709))
* restrict sandbox CPU affinity to one core via max_cpus ([2ca4e57](https://github.com/codize-dev/sandbox/commit/2ca4e5726911d27f53d594b3b78313ba7e9d698c))
* return status "SIGNAL" when process is terminated by a signal ([a997959](https://github.com/codize-dev/sandbox/commit/a9979592c3ce2153236ab6c65d637ad3d987c596))
* separate compile and run timeouts for independent nsjail time limits ([e13f7d7](https://github.com/codize-dev/sandbox/commit/e13f7d7c4253021b5375b269a7a8f0451f7071d3))
* tune per-runtime nsjail rlimit values for tighter resource isolation ([6239f56](https://github.com/codize-dev/sandbox/commit/6239f5648d4c2b35ce9ed457afb04872b44ee929))
* use poll(2) for deterministic combined output ordering ([184c1a0](https://github.com/codize-dev/sandbox/commit/184c1a05acc9b74ef10fa323b928d97fa31f951f))


### Bug Fixes

* accept both ENOTDIR and EROFS for /lib64 write test across architectures ([f02a2b2](https://github.com/codize-dev/sandbox/commit/f02a2b20dec45fa2b5667bcb16b1276703efac7f))
* add cgroup host mode to compose for cgroup v2 compatibility ([5877703](https://github.com/codize-dev/sandbox/commit/58777032667dd9de76766d370bf24f699a896f25))
* add noexec to /tmp and nosuid/nodev to bind mounts for defense-in-depth ([286424d](https://github.com/codize-dev/sandbox/commit/286424dc12634f51936b8c308a226f1b2bd2db07))
* add nosuid/nodev to /code mount and block Landlock syscalls ([ea4626a](https://github.com/codize-dev/sandbox/commit/ea4626aaeadf4ac7b5fee6b7b7028132c03fa1c0))
* add nosuid/nodev to /etc/alternatives mount and block pidfd_getfd syscall ([a10a600](https://github.com/codize-dev/sandbox/commit/a10a6003136379e2ab92fcdc212e475d1e5ae741))
* adjust large_file e2e test to respect max-file-size limit ([51f879d](https://github.com/codize-dev/sandbox/commit/51f879deac2f7bdc627e21da40a5a85db7316ce6))
* block 6 additional syscalls in seccomp policy (S-4 through S-8) ([754fa7f](https://github.com/codize-dev/sandbox/commit/754fa7fba747446d76f49de808e015085517b1f9))
* block clone/clone3 namespace creation to prevent unshare bypass ([5d6dbe7](https://github.com/codize-dev/sandbox/commit/5d6dbe7a6f87ff6f8cbd1ec3faaff49c5ec28dbc))
* block fanotify_init and fanotify_mark syscalls to prevent filesystem event snooping ([0779d4b](https://github.com/codize-dev/sandbox/commit/0779d4b854dc7a659e24141a1d3cba7959873903))
* block name_to_handle_at syscall to prevent host filesystem layout leak ([e475396](https://github.com/codize-dev/sandbox/commit/e4753964ba1729dad78e7896db3007553c5e9e98))
* improve UID/GID mapping comment accuracy and harden SUID e2e tests ([e1fa2ee](https://github.com/codize-dev/sandbox/commit/e1fa2eed8c7fac6b2d2cf7a18d492d16e29e492c))
* pin alpine base image to digest for reproducible builds ([8149085](https://github.com/codize-dev/sandbox/commit/8149085790828c68ddc62b72de1f963a5c1896b3))
* set rlimit_nproc to soft to avoid cross-sandbox interference ([aa5fb11](https://github.com/codize-dev/sandbox/commit/aa5fb1138f1e5a141aa01ad6e4970a7049d7795d))
* suppress errcheck warnings for deferred os.RemoveAll calls ([33c890f](https://github.com/codize-dev/sandbox/commit/33c890f99ebf866a23ba2e7841f1bc5bf4237877))
* Update base image ([c0b3acd](https://github.com/codize-dev/sandbox/commit/c0b3acdedecd1f8c3b34794912e034cd29ecb704))
