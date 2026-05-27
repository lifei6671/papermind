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

export function PlatformLoginPage({ api = authApi }: PlatformLoginPageProps) {
  const navigate = useNavigate();
  const { signIn } = useSession();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!username.trim()) {
      setError("请输入平台管理员账号");
      return;
    }

    if (!password.trim()) {
      setError("请输入平台管理员密码");
      return;
    }

    setIsSubmitting(true);
    setError("");
    try {
      // 平台管理员登录成功后以服务端会话为准，本地只保存后续 API 鉴权需要的 token 和身份摘要。
      const session = await api.platformLogin({
        username: username.trim(),
        password,
      });
      signIn(session);
      navigate("/", { replace: true });
    } catch (err) {
      setError(formatApiErrorMessage(err, "平台管理员登录失败"));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <main className="platform-login">
      <section className="platform-login__form-pane">
        <form className="platform-login__form" onSubmit={handleSubmit}>
          <div className="platform-login__brand">
            <BookOpenCheck aria-hidden="true" size={28} />
            <span>PaperMind</span>
          </div>

          <div>
            <h1>平台管理员登录</h1>
            <p>进入租户、注册开关和平台级配置的统一管理入口。</p>
          </div>

          {error && <div role="alert" className="platform-login__error">{error}</div>}

          <label className="auth-field">
            <span>账号</span>
            <input
              autoComplete="username"
              onChange={(event) => setUsername(event.target.value)}
              placeholder="请输入平台管理员账号"
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
            {isSubmitting ? "登录中" : "登录平台"}
          </Button>
        </form>
      </section>

      <section className="platform-login__visual" aria-label="平台管理摘要">
        <div>
          <span>Platform Console</span>
          <strong>租户码、注册开关、空间入口</strong>
          <p>平台管理员先确认租户归属，再放行用户注册与考试业务配置。</p>
        </div>
      </section>
    </main>
  );
}
