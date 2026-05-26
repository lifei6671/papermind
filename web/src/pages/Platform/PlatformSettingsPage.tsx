import { useState } from "react";
import { Panel } from "../../components/ui/Panel";
import { SectionHeader } from "../../components/ui/SectionHeader";

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
      <SectionHeader
        description="维护新租户默认注册策略、基础密码策略和前端跨域来源。"
        title="平台配置"
      />

      <Panel title="首版平台配置" subtitle="配置变更会影响后续创建租户和前端请求边界。">
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

          <button className="primary-button" type="submit">保存平台配置</button>
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
