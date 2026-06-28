# Auth Conflict Regression Results - 2026-06-26

## 1. Scope

目标：关闭注册接口唯一约束错误信息缺口，确保重复注册返回稳定、干净的客户端错误，不暴露数据库 constraint、SQLSTATE 或底层 insert 细节。

新增脚本：

```text
scripts/regression/auth-conflict.sh
```

覆盖范围：

```text
Health check
Initial user registration
Duplicate registration conflict
No database implementation detail leakage
```

## 2. Environment

```text
BASE_URL=http://localhost:8081
backend=aag-backend
postgres=aag-postgres
redis=aag-redis
frontend=http://localhost:5175
```

## 3. Command

```bash
BASE_URL=http://localhost:8081 scripts/regression/auth-conflict.sh
```

## 4. Result Summary

```text
Total: 4
Passed: 4
Failed: 0
```

## 5. Test Cases

| Test ID | Area | Target | Expected | Result |
|---|---|---|---|---:|
| TC-REG-AUTH-CONFLICT-001 | Health | Backend health | HTTP 200, `status=ok` | Pass |
| TC-REG-AUTH-CONFLICT-002 | Auth | Initial registration | HTTP 201, JWT and user returned | Pass |
| TC-REG-AUTH-CONFLICT-003 | Auth | Duplicate registration | HTTP 409, `code=conflict`, `type=client_error`, stable message | Pass |
| TC-REG-AUTH-CONFLICT-004 | Auth | Error hygiene | Message does not include constraint, SQLSTATE, insert details, pgx/pq details | Pass |

## 6. Implementation Notes

Backend changes:

```text
storage.CreateUser now maps PostgreSQL unique_violation (23505) to ErrUserAlreadyExists.
gateway.userRegister maps ErrUserAlreadyExists to HTTP 409 with a stable message.
errorTypeForCode maps conflict/not_found to client_error instead of internal_error.
unexpected user creation failures are logged server-side and returned as generic internal_error.
```

## 7. Conclusion

注册冲突错误信息缺口已关闭并固化为可重复执行脚本：

```text
scripts/regression/auth-conflict.sh
```

当前结果为 4 pass / 0 fail。
