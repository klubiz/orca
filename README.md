# Orca

**Kubernetes-inspired AI Agent Orchestration System**

Orca는 Kubernetes의 컨트롤 플레인 패턴에서 영감을 받아, Claude AI 에이전트들을 오케스트레이션하는 시스템입니다. `orca serve`로 컨트롤 플레인을 띄우고, YAML 매니페스트로 에이전트 풀과 개발 태스크를 선언하면, 시스템이 자동으로 에이전트를 생성/스케줄링/실행합니다.

## 요구사항

- **Go 1.23+**
- **Claude CLI** (`claude` 명령어가 PATH에 있어야 함, [설치 가이드](https://docs.anthropic.com/en/docs/claude-code))
  - Claude 구독이 활성화되어 있어야 함 (API 키 불필요)

## 설치

```bash
git clone https://github.com/klubiz/orca.git
cd orca
make install    # /usr/local/bin/orca 에 설치됨
```

설치 확인:

```bash
orca --help
```

## 시작하기 (5분 가이드)

### Step 1. 서버 시작

터미널을 열고 서버를 시작합니다:

```bash
orca serve
```

```
Orca Control Plane
   API Server: http://127.0.0.1:7117
   Data Dir:   ~/.orca/data
   DB Path:    ~/.orca/data/orca.db
```

서버는 포그라운드에서 실행됩니다. **새 터미널을 열어서** 아래 명령어들을 실행하세요.

### Step 2. 프로젝트 + 에이전트 풀 생성

예제 매니페스트로 한번에 세팅합니다:

```bash
orca apply -f examples/project.yaml       # 프로젝트 생성
orca apply -f examples/agentpool.yaml      # 에이전트 3개 배포
```

에이전트가 Ready 상태가 될 때까지 잠시 기다립니다 (약 3초):

```bash
orca get agentpods -p my-erp
```

```
NAME                  PROJECT  MODEL          PHASE  ACTIVE-TASKS  AGE
coding-team-49e9300c  my-erp   claude-sonnet  Ready  0             3s
coding-team-846b5b69  my-erp   claude-sonnet  Ready  0             3s
coding-team-b9683579  my-erp   claude-sonnet  Ready  0             3s
```

### Step 3. 태스크 실행

가장 간단한 방법 - 원샷 태스크:

```bash
orca run -p my-erp -- "Go로 피보나치 함수 작성해줘"
```

```
Task run-1772078604079 created. Waiting for completion...
...........................
Task Succeeded
------------------------------------------------------------
## fibonacci.go

(피보나치 함수 코드 + 테스트 코드 + 설명)
```

YAML 매니페스트로 태스크를 제출할 수도 있습니다:

```bash
orca apply -f examples/devtask.yaml
orca get devtasks -p my-erp                # 상태 확인
orca describe devtask <name> -p my-erp     # 결과 확인
```

### Step 4. 상태 확인

```bash
orca status
```

```
Orca Control Plane Status
========================

Projects: 1
Agent Pods: 3 total (3 ready)
Agent Pools: 1
Dev Tasks: 1 total (1 succeeded)
```

## 주요 사용법

### 원샷 태스크 (가장 간단)

```bash
orca run -p my-erp -- "JWT 인증 미들웨어를 Go로 작성해줘"
orca run -p my-erp -- "이 SQL 쿼리를 최적화해줘: SELECT * FROM users WHERE..."
orca run -p my-erp --model claude-opus -- "복잡한 아키텍처 설계해줘"
```

### YAML로 태스크 제출

```yaml
# task.yaml
apiVersion: orca.dev/v1alpha1
kind: DevTask
metadata:
  name: implement-search
  project: my-erp
spec:
  prompt: "고객 검색 기능을 페이지네이션과 함께 구현해줘"
  requiredCapabilities: [code]
  maxRetries: 3
  timeoutSeconds: 300
```

```bash
orca apply -f task.yaml
orca get devtasks -p my-erp              # Pending → Running → Succeeded
orca describe devtask implement-search -p my-erp   # 결과 확인
```

### 태스크 의존성 체인

여러 태스크를 순서대로 실행:

```yaml
# pipeline.yaml
apiVersion: orca.dev/v1alpha1
kind: DevTask
metadata:
  name: design-api
  project: my-erp
spec:
  prompt: "REST API 설계해줘"
  requiredCapabilities: [code]
---
apiVersion: orca.dev/v1alpha1
kind: DevTask
metadata:
  name: implement-api
  project: my-erp
spec:
  prompt: "설계된 API를 구현해줘"
  requiredCapabilities: [code]
  dependsOn: [design-api]
---
apiVersion: orca.dev/v1alpha1
kind: DevTask
metadata:
  name: write-tests
  project: my-erp
spec:
  prompt: "API 테스트를 작성해줘"
  requiredCapabilities: [test]
  dependsOn: [implement-api]
```

```bash
orca apply -f pipeline.yaml
# design-api → implement-api → write-tests 순서로 자동 실행
```

### 에이전트 스케일링

```bash
orca scale agentpool coding-team --replicas=5 -p my-erp   # 5개로 증설
orca scale agentpool coding-team --replicas=1 -p my-erp   # 1개로 축소
orca get agentpods -p my-erp                               # 확인
```

### 즉석 프롬프트

실행 중인 에이전트에 직접 프롬프트:

```bash
orca exec <pod-name> -p my-erp -- "이 코드를 리뷰해줘"
```

## CLI 명령어 전체 목록

| 명령어 | 설명 |
|--------|------|
| `orca serve` | 컨트롤 플레인 시작 |
| `orca apply -f <file>` | 리소스 생성/업데이트 (YAML) |
| `orca get <type> -p <project>` | 리소스 목록 (`agentpods`, `agentpools`, `devtasks`, `projects`) |
| `orca describe <type> <name> -p <project>` | 리소스 상세 정보 |
| `orca delete <type> <name> -p <project>` | 리소스 삭제 |
| `orca run -p <project> -- "prompt"` | 원샷 태스크 실행 |
| `orca exec <pod> -p <project> -- "prompt"` | 즉석 프롬프트 |
| `orca scale agentpool <name> --replicas=N` | 에이전트 스케일링 |
| `orca logs <pod> -p <project>` | 에이전트 로그 |
| `orca status` | 클러스터 대시보드 |
| `orca init <name>` | 프로젝트 스캐폴딩 |

모든 명령어는 `--output json|yaml|table` 포맷을 지원합니다.

## 커스텀 에이전트 풀 만들기

```yaml
apiVersion: orca.dev/v1alpha1
kind: Project
metadata:
  name: my-project
spec:
  description: "내 프로젝트"
---
apiVersion: orca.dev/v1alpha1
kind: AgentPool
metadata:
  name: my-agents
  project: my-project
spec:
  replicas: 2
  selector:
    role: developer
  template:
    metadata:
      labels:
        role: developer
    spec:
      model: claude-sonnet           # claude-sonnet | claude-opus | claude-haiku
      systemPrompt: "당신은 숙련된 백엔드 개발자입니다."
      capabilities: [code, test]
      maxConcurrency: 2
      maxTokens: 8192
      tools: [read_file, write_file, run_command]
      restartPolicy: Always
```

```bash
orca apply -f my-pool.yaml
orca run -p my-project -- "원하는 작업"
```

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
  └──────> State Store <── Claude CLI
           (BoltDB)       (local subscription)
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

### Reconciliation Loop

각 컨트롤러는 K8s와 동일한 패턴으로 동작합니다:

1. Store에서 Watch → 변경 이벤트 감지
2. WorkQueue에 추가 (중복 제거)
3. Worker가 큐에서 꺼내 `Reconcile()` 호출
4. Desired vs Actual 비교 → 차이 해소
5. 실패 시 exponential backoff으로 재시도

### Scheduler

```
DevTask(Pending) → Predicates(필터) → Priorities(스코어) → Best AgentPod
```

**Predicates:** PodIsReady, PodHasCapacity, PodMatchesCapability, PodMatchesModel, PodInSameProject

**Priorities:** LeastLoaded, CapabilityMatch, ModelPreference

## Development

```bash
make build      # 빌드
make install    # /usr/local/bin 에 설치
make test       # 테스트 실행
make run        # 빌드 후 서버 시작
make lint       # fmt + vet
```

## License

MIT
