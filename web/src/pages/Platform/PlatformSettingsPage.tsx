import { Button } from "../../components/ui/Button";
import { useState } from "react";
import { Panel } from "../../components/ui/Panel";

export function PlatformSettingsPage() {
  const [allowRegisterDefault, setAllowRegisterDefault] = useState(true);
  const [passwordMinLength, setPasswordMinLength] = useState(8);
  const [corsOrigins, setCorsOrigins] = useState("http://localhost:5173");
  const [savedSettings, setSavedSettings] = useState({
    allowRegisterDefault: true,
    passwordMinLength: 8,
    corsOrigins: "http://localhost:5173",
  });

  function handleSave(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // 平台配置只保存非密钥类首版配置，生产密钥仍通过环境变量或部署平台注入。
    setSavedSettings({
      allowRegisterDefault,
      passwordMinLength,
      corsOrigins,
    });
  }

  return (
    <section className="page platform-page">
      <nav aria-label="平台配置菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/platform-settings" role="tab" aria-selected="true">
          平台配置
        </a>
      </nav>

      <Panel>
        <form className="platform-settings-form" onSubmit={handleSave}>
          <label className="platform-check">
            <input
              checked={allowRegisterDefault}
              onChange={(event) => setAllowRegisterDefault(event.target.checked)}
              type="checkbox"
            />
            新租户默认允许自注册
          </label>

          <label className="field">
            <span>密码最小长度</span>
            <input
              min={6}
              onChange={(event) => setPasswordMinLength(Number(event.target.value))}
              type="number"
              value={passwordMinLength}
            />
          </label>

          <label className="field">
            <span>CORS 允许来源</span>
            <textarea
              onChange={(event) => setCorsOrigins(event.target.value)}
              value={corsOrigins}
            />
          </label>

          <Button variant="primary" type="submit">保存平台配置</Button>
        </form>
      </Panel>

      <div className="platform-settings-summary" role="status">
        <strong>当前生效配置</strong>
        <span>{savedSettings.allowRegisterDefault ? "默认允许自注册" : "默认关闭自注册"}</span>
        <span>密码至少 {savedSettings.passwordMinLength} 位</span>
        <span>{savedSettings.corsOrigins}</span>
      </div>
    </section>
  );
}
