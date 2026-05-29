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
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

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
        username: username.trim(),
        password,
      });
      signIn(session);
      navigate("/tenant-entry", { replace: true });
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
            <p>{mode === "platform" ? "进入租户、注册开关和平台级配置的统一管理入口。" : "登录后选择要进入的租户空间。"}</p>
          </div>

          {error && <div role="alert" className="platform-login__error">{error}</div>}

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
          <span>{mode === "platform" ? "Platform Console" : "Tenant Spaces"}</span>
          <strong>{mode === "platform" ? "租户码、注册开关、空间入口" : "通用账号、租户关系、空间入口"}</strong>
          <p>{mode === "platform" ? "平台管理员先确认租户归属，再放行用户注册与考试业务配置。" : "租户用户先完成账号登录，再从可用空间中选择本次进入的上下文。"}</p>
        </div>
      </section>
    </main>
  );
}
