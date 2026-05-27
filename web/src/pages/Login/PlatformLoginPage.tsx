import { Button } from "../../components/ui/Button";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { BookOpenCheck } from "lucide-react";
import { useSession } from "../../auth/session-context";
import { authApi } from "../../api/auth";
import type { AuthAPI } from "../../api/auth";
import { formatApiErrorMessage } from "../../api/client";

type PlatformLoginPageProps = {
  api?: AuthAPI;
};

type LoginMode = "platform" | "tenant";

export function PlatformLoginPage({ api = authApi }: PlatformLoginPageProps) {
  const navigate = useNavigate();
  const { signIn } = useSession();
  const [mode, setMode] = useState<LoginMode>("platform");
  const [tenantID, setTenantID] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const normalizedTenantID = Number(tenantID);
    if (mode === "tenant" && (!Number.isInteger(normalizedTenantID) || normalizedTenantID <= 0)) {
      setError("请输入租户 ID");
      return;
    }

    if (!username.trim()) {
      setError(mode === "platform" ? "请输入平台管理员账号" : "请输入租户账号");
      return;
    }

    if (!password.trim()) {
      setError(mode === "platform" ? "请输入平台管理员密码" : "请输入租户密码");
      return;
    }

    setIsSubmitting(true);
    setError("");
    try {
      if (mode === "platform") {
        // 平台管理员登录成功后以服务端会话为准，本地只保存后续 API 鉴权需要的 token 和身份摘要。
        const session = await api.platformLogin({
          username: username.trim(),
          password,
        });
        signIn(session);
        navigate("/", { replace: true });
        return;
      }

      const session = await api.tenantLogin({
        tenantID: normalizedTenantID,
        username: username.trim(),
        password,
      });
      signIn(session);
      navigate(session.user.role === "student" ? "/exam-entry" : "/", { replace: true });
    } catch (err) {
      setError(formatApiErrorMessage(err, mode === "platform" ? "平台管理员登录失败" : "租户用户登录失败"));
    } finally {
      setIsSubmitting(false);
    }
  }

  function switchMode(nextMode: LoginMode) {
    setMode(nextMode);
    setError("");
  }

  return (
    <main className="platform-login">
      <section className="platform-login__form-pane">
        <form className="platform-login__form" onSubmit={handleSubmit}>
          <div className="platform-login__brand">
            <BookOpenCheck aria-hidden="true" size={28} />
            <span>PaperMind</span>
          </div>

          <div className="platform-login__mode-switch" role="tablist" aria-label="登录类型">
            <button
              aria-selected={mode === "platform"}
              onClick={() => switchMode("platform")}
              role="tab"
              type="button"
            >
              平台管理员
            </button>
            <button
              aria-selected={mode === "tenant"}
              onClick={() => switchMode("tenant")}
              role="tab"
              type="button"
            >
              租户用户
            </button>
          </div>

          <div>
            <h1>{mode === "platform" ? "平台管理员登录" : "租户用户登录"}</h1>
            <p>{mode === "platform" ? "进入租户、注册开关和平台级配置的统一管理入口。" : "学生登录后可通过邀请码进入考试入口。"}</p>
          </div>

          {error && <div role="alert" className="platform-login__error">{error}</div>}

          {mode === "tenant" && (
            <label className="auth-field">
              <span>租户 ID</span>
              <input
                autoComplete="organization"
                inputMode="numeric"
                onChange={(event) => setTenantID(event.target.value)}
                placeholder="请输入租户 ID"
                value={tenantID}
              />
            </label>
          )}

          <label className="auth-field">
            <span>{mode === "platform" ? "账号" : "租户账号"}</span>
            <input
              autoComplete="username"
              onChange={(event) => setUsername(event.target.value)}
              placeholder={mode === "platform" ? "请输入平台管理员账号" : "请输入租户账号"}
              value={username}
            />
          </label>

          <label className="auth-field">
            <span>密码</span>
            <input
              autoComplete="current-password"
              onChange={(event) => setPassword(event.target.value)}
              placeholder="请输入密码"
              type="password"
              value={password}
            />
          </label>

          <Button className="auth-submit" disabled={isSubmitting} type="submit">
            {isSubmitting ? "登录中" : mode === "platform" ? "登录平台" : "登录租户"}
          </Button>
        </form>
      </section>

      <section className="platform-login__visual" aria-label={mode === "platform" ? "平台管理摘要" : "考试入口摘要"}>
        <div>
          <span>{mode === "platform" ? "Platform Console" : "Student Exam"}</span>
          <strong>{mode === "platform" ? "租户码、注册开关、空间入口" : "租户账号、邀请码、考试入口"}</strong>
          <p>{mode === "platform" ? "平台管理员先确认租户归属，再放行用户注册与考试业务配置。" : "考生先以租户学生账号登录，再填写邀请码进入独立考试端。"}</p>
        </div>
      </section>
    </main>
  );
}
