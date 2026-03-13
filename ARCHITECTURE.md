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

## Startup Flow

1. Load env config.
2. Bootstrap runtime keys vào etcd nếu thiếu.
3. Reload runtime config từ etcd.
4. Build services + transports.
5. Start HTTP server và gRPC endpoints.
