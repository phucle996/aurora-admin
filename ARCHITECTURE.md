# Architecture

## Role

`aurora-admin` là control plane:

- quản lý runtime config
- cấp cert/bootstrap cho agent
- điều phối module install metadata
- expose UI + API cho vận hành

## Main Components

- `cmd/server`: entrypoint.
- `internal/app`: wiring, route, middleware.
- `internal/service`: business/service layer.
- `internal/repository`: etcd/cert-store abstraction.
- `internal/transport/http`: admin APIs.
- `internal/transport/grpc`: runtime bootstrap + agent RPC.
- `src`: React/Vite UI (embed vào binary).

## Security Model (current)

- Admin edge TLS và Agent mTLS CA tách riêng.
- Agent enroll bằng bootstrap token + CSR.
- Sau enroll, agent dùng mTLS.
- Identity nằm trong cert SAN claim (node/service/role/cluster).
- Runtime authz theo role ACL ở gRPC layer.

## Discovery

- Agent presence lưu trong etcd theo lease TTL tại `/registry/agents/<agent_id>`.
- Admin list/resolve live agent từ registry (không phụ thuộc `/etc/hosts` cho nội bộ control-plane).

## Installer Direction

- Kiến trúc installer thế hệ tiếp theo được chốt ở [AURORA_INSTALLER_PHASE0.md](/home/phucle/Desktop/project/AURORA_INSTALLER_PHASE0.md).
- Roadmap production-grade được chốt ở [AURORA_INSTALLER_PRODUCTION_GRADE.md](/home/phucle/Desktop/project/AURORA_INSTALLER_PRODUCTION_GRADE.md).
- Hướng đi chính:
  - artifact chuẩn hóa
  - installer engine nằm trong agent
  - Admin chỉ orchestration

## Service Install Flow

Hiện tại flow install service trên `aurora-admin` là:

- Input công khai từ UI/API chỉ còn:
  - `module_name`
  - `agent_id`
  - `app_host`
- Runtime install luôn cố định là `linux-systemd`.
- `ums`, `platform`, `paas`, `dbaas` đi theo bundle install qua agent.
- `ui` vẫn còn đi theo legacy install script.

### Sequence

```mermaid
sequenceDiagram
    autonumber
    participant UI as UI/API
    participant Admin as Admin ModuleInstallService
    participant Registry as etcd Registry
    participant Agent as Target Agent
    participant Store as Runtime/Cert Store

    UI->>Admin: Install(module_name, agent_id, app_host)
    Admin->>Registry: resolve agent from /registry/agents/<agent_id>
    Admin->>Agent: check target port availability
    Admin->>Store: desired/actual = installing
    Admin->>Store: prepare schema + seed runtime schema key
    Admin->>Store: resolve admin gRPC endpoint if required
    Admin->>Store: preseed endpoint + app_port

    alt bundle-managed module
        Admin->>Admin: resolve latest release + checksums
        Admin->>Agent: push TLS materials
        Admin->>Agent: InstallModuleStream / InstallModule
        Agent-->>Admin: stream logs + result
        Admin->>Agent: ListInstalledModules
    else legacy path (UI)
        Admin->>Admin: build install command
        Admin->>Agent: push TLS materials
        Admin->>Agent: RunCommand(install.sh ...)
        Admin->>Agent: verify / self-heal TLS if needed
    end

    Admin->>Agent: sync /etc/hosts on target
    Admin->>Store: seed /runtime/hosts/<app_host>
    Admin->>Agent: broadcast hosts update to other agents
    opt legacy path only
        Admin->>Agent: configure nginx proxy
    end
    Admin->>Store: seed module TLS bundle
    Admin->>Store: desired/actual = installed
    Admin-->>UI: install result
```

### Orchestration Graph

```mermaid
flowchart TD
    A[Install request<br/>module_name, agent_id, app_host] --> B[Resolve target agent]
    B --> C[Generate candidate app port]
    C --> D[Check port availability on target]
    D --> E[Begin install state<br/>status=installing]
    E --> F[Prepare schema and migrations]
    F --> G[Resolve admin bootstrap endpoint]
    G --> H[Preseed endpoint and runtime app port]
    H --> I[Install module TLS on target]

    I --> J{Bundle-managed module?}
    J -->|Yes| K[Resolve latest release and checksum]
    K --> L[Call Agent InstallModuleStream]
    L --> M[Sync actual state from agent inventory]
    J -->|No| N[Build legacy install command]
    N --> O[Run install command via agent]
    O --> P[Verify or self-heal TLS]

    M --> Q[Sync target /etc/hosts]
    P --> Q
    Q --> R[Seed runtime host routing]
    R --> S[Broadcast host updates]
    S --> T{Legacy path?}
    T -->|Yes| U[Configure nginx proxy]
    T -->|No| V[Skip nginx on admin side]
    U --> W[Seed module TLS bundle]
    V --> W
    W --> X[Mark install state installed]
```

### Agent Bundle Path

`Admin` không còn cài trực tiếp binary/service cho các module bundle-managed. Thay vào đó:

```mermaid
flowchart LR
    Admin[Admin] -->|InstallModuleStream<br/>module, version, artifact_url,<br/>artifact_checksum, app_host, app_port, env| Agent[Agent]
    Agent --> D[Download artifact]
    D --> V[Verify checksum]
    V --> U[Unpack manifest.json]
    U --> R[Render env, systemd, nginx]
    R --> S[systemctl daemon-reload]
    S --> E[enable and restart service]
    E --> N[nginx -t and reload]
    N --> H[healthcheck]
    H --> I[update installed_modules.json]
    I --> Admin
```

### State Model

Install state hiện dùng vocabulary chung giữa `desired`, `actual`, và agent inventory:

- `installing`
- `installed`
- `failed`
- `missing`
- `agent-unreachable`
- `unknown`

## Startup Flow

1. Load env config.
2. Bootstrap runtime keys vào etcd nếu thiếu.
3. Reload runtime config từ etcd.
4. Build services + transports.
5. Start HTTP server và gRPC endpoints.
