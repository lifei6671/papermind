import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { BookOpenCheck } from "lucide-react";
import { useSession } from "../../auth/session-context";

export function PlatformLoginPage() {
  const navigate = useNavigate();
  const { signIn } = useSession();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // 平台管理员登录页先完成前端登录态闭环，后续真实 API 接入后只替换提交动作。
    if (!username.trim()) {
      setError("请输入平台管理员账号");
      return;
    }

    if (!password.trim()) {
      setError("请输入平台管理员密码");
      return;
    }

    signIn({
      accessToken: "platform-local-access-token",
      refreshToken: "platform-local-refresh-token",
      user: {
        displayName: "平台管理员",
        role: "platform_admin",
        userID: 1,
      },
    });
    navigate("/", { replace: true });
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

          <button className="auth-submit" type="submit">登录平台</button>
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
