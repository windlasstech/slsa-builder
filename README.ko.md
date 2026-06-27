# slsa-builder

재사용 가능하고 프로필 확장 가능한 SLSA 출처(provenance) 빌더 기반입니다.

## 개발 환경 설정

이 저장소는 개발 도구 런타임 버전을 설치하고 고정하기 위해
[mise](https://mise.jdx.dev/)를 사용합니다. Go가 주요 구현 언어이며,
Node.js와 pnpm은 Prettier나 Lefthook 같은 개발 도구 용도로만 사용합니다.

### 필수 조건

- [mise](https://mise.jdx.dev/getting-started.html) 설치
- 사용자 이름과 이메일을 설정한 Git

### 부트스트랩

```bash
mise install
pnpm install
```

이 명령은 `mise.toml`에 정의된 Go, Node.js, pnpm, CLI 도구의 고정된
버전을 설치합니다. Lefthook hook은 mise가 Lefthook을 설치할 때
`postinstall` 단계로 자동 설치됩니다. 그 후 `pnpm install` 단계에서
`package.json`에 선언된 프로젝트 로컬 개발 의존성을 설치합니다.

CI에서는 레지스트리에 대한 API 호출을 방지하기 위해 잠금 모드로 mise를
실행하세요.

```bash
MISE_LOCKED=1 mise install
pnpm install
```

부트스트랩이 완료되면 mise를 통해 다음 명령을 사용할 수 있습니다.

```bash
go version
node --version
pnpm --version
golangci-lint --version
shellcheck --version
shfmt --version
lefthook --version
actionlint --version
```

### mise와 pnpm의 담당 범위

mise는 언어 런타임과 독립 실행형 CLI 바이너리를 설치합니다.

- Go, Node.js, pnpm
- `golangci-lint`, `shellcheck`, `shfmt`, `lefthook`, `actionlint`

Go 소스 포매팅과 import 정규화는 독립 실행형 포매터 바이너리가 아닌,
`.golangci.yml`에 구성된 `golangci-lint` 포매터(`gofmt`, `goimports`)가
담당합니다.

pnpm은 저장소 설정 파일과 길게 연결된 Node.js 기반 개발 의존성을 설치합니다.

- `prettier` (`.prettierrc`로 구성)
- `markdownlint-cli2` (`.markdownlint-cli2.jsonc`로 구성)

Prettier와 `markdownlint-cli2`를 프로젝트 로컬 pnpm 의존성으로 유지하면
`pnpm-lock.yaml`에 전체 의존성 그래프가 남고, 에디터 통합 및
조직의 의존성 검토 워크플로우와 정렬됩니다.

### 도구 버전

도구 버전은 `mise.toml`에 선언되어 있습니다. 플랫폼 간 재현 가능한 설치를
보장하기 위해 `mise.lock` 파일이 커밋되어 있습니다. `mise.toml`에서 도구
버전을 변경한 경우 다음 명령으로 잠금 파일을 다시 생성하세요.

```bash
mise lock
```

### 커밋 규약 및 서명

이 프로젝트는 모든 커밋에 `Signed-off-by:` 트레일러(DCO)가 필요합니다.
Lefthook이 로컬에서 이를 강제하도록 구성되어 있으며, CI와 브랜치 보호가
권위 있는 검증을 수행합니다.
