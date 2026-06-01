import { Button } from "../../components/ui/Button";
import { Panel } from "../../components/ui/Panel";
import { useState } from "react";

type SpaceConfigPageProps = {
  tenantID?: number;
};

export function SpaceConfigPage({ tenantID }: SpaceConfigPageProps) {
  const [defaultDuration, setDefaultDuration] = useState(60);
  const [showPracticeAnalysis, setShowPracticeAnalysis] = useState(true);
  const [saveMessage, setSaveMessage] = useState("");
  const hasTenantContext = Number.isInteger(tenantID) && Number(tenantID) > 0;

  function handleSaveConfig(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    setSaveMessage("空间配置已保存");
  }

  if (!hasTenantContext) {
    return (
      <section className="page platform-page tenant-admin-page">
        <nav aria-label="空间配置菜单" className="platform-tabbar" role="tablist">
          <a className="platform-tab platform-tab--active" href="/space-config" role="tab" aria-selected="true">
            空间配置
          </a>
        </nav>

        <Panel>
          <div className="tenant-admin-warning" role="alert">
            当前账号没有租户上下文，请先登录租户账号后再进入空间配置。
          </div>
        </Panel>
      </section>
    );
  }

  return (
    <section className="page platform-page tenant-admin-page">
      <nav aria-label="空间配置菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/space-config" role="tab" aria-selected="true">
          空间配置
        </a>
      </nav>

      <Panel>
        <form className="platform-settings-form" onSubmit={handleSaveConfig}>
          <label className="field">
            <span>默认考试时长</span>
            <input
              aria-label="默认考试时长"
              min={1}
              onChange={(event) => setDefaultDuration(Number(event.target.value))}
              type="number"
              value={defaultDuration}
            />
          </label>
          <label className="platform-check">
            <input
              aria-label="允许学生查看练习解析"
              checked={showPracticeAnalysis}
              onChange={(event) => setShowPracticeAnalysis(event.target.checked)}
              type="checkbox"
            />
            允许学生查看练习解析
          </label>
          {saveMessage && <div className="tenant-admin-success" role="status">{saveMessage}</div>}
          <div className="platform-dialog__actions">
            <Button variant="primary" type="submit">
              保存空间配置
            </Button>
          </div>
        </form>
      </Panel>
    </section>
  );
}
