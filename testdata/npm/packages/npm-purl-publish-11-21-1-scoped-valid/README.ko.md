# npm PURL 호환성: 게시 npm 11.21.1, 스코프 패키지

원본: 이 디렉터리의 `@windlass/purl-compat@0.1.0` 패키지 트리.

버전 키: 게시 단계 허용 목록의 npm 11.21.1.

목적: `NPMPackagePURL`의 스코프 `%40` 주체를 고정합니다.

검증 상태: 12026-10-29 UTC에 darwin/arm64의 Go 1.27.3과 Node.js 24.22.1에서 통과했습니다.
`TestNPMPURLCLICompatibilityLive`에서 이 npm CLI에 번들된 `npm-package-arg`, `NPMPackagePURL`,
픽스처 주체가 바이트 단위로 일치함을 확인했습니다.

실행 명령: `SLSA_BUILDER_LIVE_TOOLCHAIN=1 go test -count=1 ./internal/npmprofile -run 'Live$'`
(`-short` 없이 `GOFLAGS='-mod=readonly -v'`와 명령 범위의 루프백 `HTTPS_PROXY` 사용). 증거:
baseline의 `live-toolchain.log`. 게시나 OIDC 교환은 수행하지 않았습니다.

기록일: 12026-10-30.

재생성: 패키지와 주체 바이트를 보존하고 업데이트 시 허용 목록을 검토합니다.
