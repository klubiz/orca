# Orca

**Kubernetes-inspired AI Agent Orchestration System**

Orca는 Kubernetes의 컨트롤 플레인 패턴에서 영감을 받아, Claude AI 에이전트들을 오케스트레이션하는 시스템입니다. **OR**chestration of **A**gents의 약자이며, 범고래가 pod 단위로 협업 사냥하는 것에서 착안했습니다.

`orca serve`로 컨트롤 플레인을 띄우고, YAML 매니페스트로 에이전트 풀과 개발 태스크를 선언하면, 시스템이 자동으로 에이전트를 생성/스케줄링/실행합니다.

## Architecture

```
  orca CLI (kubectl-like)
       │  HTTP/REST
       v
  API Server (:7117)
       │
  ┌────┼──────────────┐
  v    v              v
Controller   Scheduler   Agent Runtime
Manager                      │
  │                          v
  └──────> State Store <── Claude API
           (BoltDB)
```

### K8s → Orca 매핑

| K8s | Orca | 설명 |
|-----|------|------|
| Pod | AgentPod | 실행 중인 AI 에이전트 인스턴스 |
| Deployment | AgentPool | 에이전트 그룹의 desired state |
| Job | DevTask | 개발 태스크 (코딩, 테스트, 리뷰) |
| Namespace | Project | 격리 경계 |
| etcd | BoltDB | 상태 저장소 |
| kube-scheduler | Scheduler | 태스크→에이전트 배정 |
| controller-manager | Controller Manager | 리컨실리에이션 루프 |

## Quick Start

### 설치

```bash
# 소스에서 빌드
git clone https://github.com/klubiz/orca.git
cd orca
make build

# 또는 직접 설치
make install   # /usr/local/bin/orca
```

**요구사항:** Go 1.23+, `ANTHROPIC_API_KEY` 환경변수

### 사용법

#### 1. 컨트롤 플레인 시작

```bash
orca serve
```

```
Orca Control Plane
   API Server: http://127.0.0.1:7117
   Data Dir:   ~/.orca/data
   DB Path:    ~/.orca/data/orca.db
```

#### 2. 프로젝트 생성

```yaml
# project.yaml
apiVersion: orca.dev/v1alpha1
kind: Project
metadata:
  name: my-app
spec:
  description: "My Application"
```

```bash
orca apply -f project.yaml
# Project/my-app configured
```

#### 3. 에이전트 풀 배포

```yaml
# agentpool.yaml
apiVersion: orca.dev/v1alpha1
kind: AgentPool
metadata:
  name: dev-team
  project: my-app
spec:
  replicas: 3
  selector:
    role: developer
  template:
    metadata:
      labels:
        role: developer
    spec:
      model: claude-sonnet
      capabilities: [code, test, debug]
      maxConcurrency: 2
      maxTokens: 8192
      tools: [read_file, write_file, run_command]
      restartPolicy: Always
```

```bash
orca apply -f agentpool.yaml
# AgentPool/dev-team configured

orca get agentpods -p my-app
# NAME                  PROJECT  MODEL          PHASE  ACTIVE-TASKS  AGE
# dev-team-0928d142     my-app   claude-sonnet  Ready  0             3s
# dev-team-634ba331     my-app   claude-sonnet  Ready  0             3s
# dev-team-6822082f     my-app   claude-sonnet  Ready  0             3s
```

#### 4. 태스크 제출

```yaml
# devtask.yaml
apiVersion: orca.dev/v1alpha1
kind: DevTask
metadata:
  name: implement-search
  project: my-app
spec:
  prompt: "Implement customer search with pagination"
  requiredCapabilities: [code]
  maxRetries: 3
  timeoutSeconds: 300
```

```bash
orca apply -f devtask.yaml
# DevTask/implement-search configured

orca get devtasks -p my-app
# NAME              PROJECT  PHASE    ASSIGNED-POD       RETRIES  AGE
# implement-search  my-app   Running  dev-team-0928d142  0        5s
```

#### 5. 상태 확인

```bash
orca status
# Orca Control Plane Status
# ========================
# Projects: 1
# Agent Pods: 3 total (2 ready, 1 busy)
# Agent Pools: 1
# Dev Tasks: 1 total (1 running)
```

## CLI Commands

```bash
orca serve                             # 컨트롤 플레인 시작
orca apply -f manifest.yaml            # 리소스 생성/업데이트
orca get pods|pools|tasks|projects     # 리소스 목록 조회
orca describe <type> <name>            # 상세 정보
orca delete <type> <name>              # 리소스 삭제
orca logs <podname>                    # 에이전트 로그
orca exec <podname> -- "prompt"        # 즉석 프롬프트 실행
orca run --model sonnet -- "prompt"    # 원샷 태스크
orca scale agentpool <name> --replicas=N
orca status                            # 클러스터 대시보드
orca init [project-name]               # 프로젝트 스캐폴딩
```

모든 커맨드는 `--output json|yaml|table` 포맷을 지원합니다.

## Resource Types

### AgentPod

실행 중인 AI 에이전트 인스턴스. 라이프사이클:

```
Pending → Starting → Ready ↔ Busy → Terminating → Terminated
                       ↓
                    Failed (heartbeat 만료 시)
```

### AgentPool

AgentPod 그룹의 desired state를 선언. replicas 수에 맞춰 자동으로 Pod을 생성/삭제합니다.

### DevTask

AI 에이전트가 실행할 개발 태스크. 라이프사이클:

```
Pending → Scheduled → Running → Succeeded/Failed
   ↑                              │ (retry)
   └──────────────────────────────┘
```

`dependsOn`으로 태스크 간 의존성 체인을 구성할 수 있습니다.

## Key Patterns

### Reconciliation Loop

각 컨트롤러는 K8s와 동일한 패턴으로 동작합니다:

1. Store에서 Watch → 변경 이벤트 감지
2. WorkQueue에 추가 (중복 제거)
3. Worker가 큐에서 꺼내 `Reconcile()` 호출
4. Desired vs Actual 비교 → 차이 해소
5. 실패 시 exponential backoff으로 재시도 (1s → 2s → 4s → ... → 60s)

### Scheduler

```
DevTask(Pending) → Predicates(필터) → Priorities(스코어) → Best AgentPod
```

**Predicates** (모두 통과해야 함):
- `PodIsReady` - Ready 상태인지
- `PodHasCapacity` - 동시 작업 여유가 있는지
- `PodMatchesCapability` - 필요한 capability가 있는지
- `PodMatchesModel` - 모델이 일치하는지
- `PodInSameProject` - 같은 프로젝트인지

**Priorities** (점수 합산, 높을수록 선호):
- `LeastLoaded` - 부하가 적은 Pod 선호
- `CapabilityMatch` - capability 매칭 점수
- `ModelPreference` - 모델 일치 보너스

## Project Structure

```
orca/
├── cmd/orca/main.go               # 엔트리포인트
├── internal/
│   ├── apiserver/                  # REST API 서버 (gorilla/mux)
│   ├── cli/                       # CLI 커맨드 (cobra)
│   ├── controller/                # 리컨실리에이션 컨트롤러
│   ├── scheduler/                 # Predicate + Priority 스케줄러
│   ├── agent/                     # Claude API 통합 + 에이전트 런타임
│   ├── store/                     # BoltDB + 인메모리 저장소
│   └── config/                    # 설정 관리
├── pkg/
│   ├── apis/v1alpha1/types.go     # 모든 리소스 타입 정의
│   ├── manifest/parser.go         # YAML 매니페스트 파서
│   └── client/client.go           # API 클라이언트 라이브러리
└── examples/                      # 예제 매니페스트
```

## Dependencies

| Library | Purpose |
|---------|---------|
| [cobra](https://github.com/spf13/cobra) | CLI 프레임워크 |
| [bbolt](https://go.etcd.io/bbolt) | 임베디드 KV 스토어 |
| [anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) | Claude API |
| [gorilla/mux](https://github.com/gorilla/mux) | HTTP 라우터 |
| [zap](https://go.uber.org/zap) | 구조화 로깅 |
| [color](https://github.com/fatih/color) | 터미널 컬러 |
| [yaml.v3](https://gopkg.in/yaml.v3) | YAML 파싱 |
| [uuid](https://github.com/google/uuid) | UUID 생성 |

## Development

```bash
make build      # 빌드
make test       # 테스트 실행
make fmt        # 코드 포맷팅
make vet        # 정적 분석
make lint       # fmt + vet
make run        # 빌드 후 서버 시작
```

## License

MIT
