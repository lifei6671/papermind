import { DataTable } from "../../components/ui/DataTable";
import { MetricCard } from "../../components/ui/MetricCard";
import { Panel } from "../../components/ui/Panel";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { StatusBadge } from "../../components/ui/StatusBadge";

const riskRows = [
  {
    scene: "阅卷",
    owner: "教师端",
    status: <StatusBadge tone="warning">待接入</StatusBadge>,
    next: "接入 P8 待阅卷列表",
  },
  {
    scene: "成绩导出",
    owner: "考试负责人",
    status: <StatusBadge tone="success">服务已就绪</StatusBadge>,
    next: "补齐前端导出按钮",
  },
  {
    scene: "考试端",
    owner: "考生",
    status: <StatusBadge tone="info">规划中</StatusBadge>,
    next: "实现邀请码入口和自动保存提示",
  },
];

export function DashboardPage() {
  return (
    <section className="page">
      <SectionHeader
        action={<button className="primary-button" type="button">新建考试</button>}
        description="围绕租户、题库、组卷、考试、阅卷和成绩发布组织首版管理端。"
        title="考试平台概览"
      />

      <div className="dashboard-summary">
        <MetricCard label="租户" value="3" trend="平台管理员视角" />
        <MetricCard label="进行中考试" value="8" trend="含草稿、已发布和阅卷中" />
        <MetricCard label="待阅卷答卷" value="26" trend="简答题人工评分入口" />
        <MetricCard label="可导出成绩" value="12" trend="P8 导出服务已完成" />
      </div>

      <div className="page-grid page-grid--two">
        <Panel title="P9 迁移状态" subtitle="本轮先迁入 axiom-ui 管理端 UI 骨架。">
          <div className="stack-list">
            <div>
              <strong>React Router</strong>
              <span>已建立 Papermind 管理端路由元数据。</span>
            </div>
            <div>
              <strong>管理端 Shell</strong>
              <span>复用 axiom-ui 固定侧栏、工作区和紧凑导航。</span>
            </div>
            <div>
              <strong>业务页面</strong>
              <span>先保留占位页，后续逐项接入真实 API。</span>
            </div>
          </div>
        </Panel>

        <Panel title="下一批页面" subtitle="围绕考试平台的真实工作流继续补齐。">
          <DataTable
            columns={[
              { key: "scene", label: "场景" },
              { key: "owner", label: "角色" },
              { key: "status", label: "状态" },
              { key: "next", label: "下一步" },
            ]}
            rows={riskRows}
          />
        </Panel>
      </div>
    </section>
  );
}
