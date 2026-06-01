import type { ReactNode } from "react";
import type { ProfileSpaceAuthorization, SessionUser } from "../../auth/session-context";
import { Button } from "../../components/ui/Button";
import { DataTable } from "../../components/ui/DataTable";
import { MetricCard } from "../../components/ui/MetricCard";
import { Panel } from "../../components/ui/Panel";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { StatusBadge } from "../../components/ui/StatusBadge";

type DashboardPageProps = {
  profileSpaces?: ProfileSpaceAuthorization[];
  selectedSpaceID?: number;
  user?: SessionUser | null;
};

type DashboardMetric = {
  label: string;
  trend: string;
  value: string;
};

type DashboardRow = {
  scene: string;
  owner: string;
  status: ReactNode;
  next: string;
};

type DashboardContent = {
  actionLabel: string;
  description: string;
  metrics: DashboardMetric[];
  rows: DashboardRow[];
  statusSubtitle: string;
  statusTitle: string;
  tableSubtitle: string;
  tableTitle: string;
  title: string;
};

export function DashboardPage({ profileSpaces = [], selectedSpaceID, user }: DashboardPageProps) {
  const content = buildDashboardContent(user, selectedSpaceID, profileSpaces);

  return (
    <section className="page">
      <SectionHeader
        action={<Button variant="primary" type="button">{content.actionLabel}</Button>}
        description={content.description}
        title={content.title}
      />

      <div className="dashboard-summary">
        {content.metrics.map((metric) => (
          <MetricCard key={metric.label} label={metric.label} value={metric.value} trend={metric.trend} />
        ))}
      </div>

      <div className="page-grid page-grid--two">
        <Panel title={content.statusTitle} subtitle={content.statusSubtitle}>
          <div className="stack-list">
            {content.rows.map((row) => (
              <div key={row.scene}>
                <strong>{row.scene}</strong>
                <span>{row.next}</span>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title={content.tableTitle} subtitle={content.tableSubtitle}>
          <DataTable
            columns={[
              { key: "scene", label: "场景" },
              { key: "owner", label: "角色" },
              { key: "status", label: "状态" },
              { key: "next", label: "下一步" },
            ]}
            rows={content.rows}
          />
        </Panel>
      </div>
    </section>
  );
}

function buildDashboardContent(
  user: SessionUser | null | undefined,
  selectedSpaceID: number | undefined,
  profileSpaces: ProfileSpaceAuthorization[],
): DashboardContent {
  const currentSpace = currentEnabledSpace(user, selectedSpaceID, profileSpaces);
  const tenantName = currentSpace?.tenantName ?? profileSpaces.find((space) => space.tenantID === user?.tenantID)?.tenantName;
  const spaceName = currentSpace?.spaceName ?? (selectedSpaceID ? `空间 ${selectedSpaceID}` : undefined);

  if (user?.role === "teacher") {
    return teacherDashboard(spaceName);
  }

  if (user?.role === "tenant_admin" || currentSpace?.role === "space_admin") {
    return tenantDashboard(spaceName, tenantName);
  }

  return platformDashboard();
}

function currentEnabledSpace(
  user: SessionUser | null | undefined,
  selectedSpaceID: number | undefined,
  profileSpaces: ProfileSpaceAuthorization[],
) {
  if (!selectedSpaceID) {
    return undefined;
  }
  return profileSpaces.find((space) =>
    space.status === "enabled" &&
    space.spaceID === selectedSpaceID &&
    (user?.tenantID === undefined || space.tenantID === user.tenantID),
  );
}

function platformDashboard(): DashboardContent {
  return {
    actionLabel: "新建考试",
    description: "围绕租户、题库、组卷、考试、阅卷和成绩发布组织首版管理端。",
    metrics: [
      { label: "租户", value: "3", trend: "平台管理员视角" },
      { label: "进行中考试", value: "8", trend: "含草稿、已发布和阅卷中" },
      { label: "待阅卷答卷", value: "26", trend: "简答题人工评分入口" },
      { label: "可导出成绩", value: "12", trend: "P8 导出服务已完成" },
    ],
    rows: [
      { scene: "阅卷", owner: "教师端", status: <StatusBadge tone="warning">待接入</StatusBadge>, next: "接入 P8 待阅卷列表" },
      { scene: "成绩导出", owner: "考试负责人", status: <StatusBadge tone="success">服务已就绪</StatusBadge>, next: "补齐前端导出按钮" },
      { scene: "考试端", owner: "考生", status: <StatusBadge tone="info">规划中</StatusBadge>, next: "实现邀请码入口和自动保存提示" },
    ],
    statusSubtitle: "本轮先迁入 axiom-ui 管理端 UI 骨架。",
    statusTitle: "P9 迁移状态",
    tableSubtitle: "围绕考试平台的真实工作流继续补齐。",
    tableTitle: "下一批页面",
    title: "考试平台概览",
  };
}

function tenantDashboard(spaceName: string | undefined, tenantName: string | undefined): DashboardContent {
  const scopeName = spaceName ?? tenantName ?? "租户空间";

  return {
    actionLabel: "新建考试",
    description: spaceName
      ? "围绕当前空间的题库、组卷、考试、阅卷和成绩发布组织管理任务。"
      : "围绕本租户的空间、题库、考试、阅卷和成绩发布组织管理任务。",
    metrics: [
      { label: spaceName ? "当前空间" : "租户空间", value: spaceName ? "1" : "3", trend: "租户管理员视角" },
      { label: "空间考试", value: "6", trend: "本空间题库与考试发布" },
      { label: "待阅卷答卷", value: "18", trend: "需协调教师处理" },
      { label: "可发布成绩", value: "9", trend: "本空间成绩发布" },
    ],
    rows: [
      { scene: "空间题库", owner: "租户管理员", status: <StatusBadge tone="success">可维护</StatusBadge>, next: "维护本空间题目与导入任务" },
      { scene: "考试发布", owner: "租户管理员", status: <StatusBadge tone="info">进行中</StatusBadge>, next: "配置考试范围与邀请码" },
      { scene: "成绩发布", owner: "租户管理员", status: <StatusBadge tone="success">可发布</StatusBadge>, next: "检查成绩策略并导出 CSV" },
    ],
    statusSubtitle: "当前概览聚焦租户管理员可管理的空间内工作。",
    statusTitle: "空间管理状态",
    tableSubtitle: "按当前空间整理租户管理员的下一步操作。",
    tableTitle: "空间待办",
    title: `${scopeName}概览`,
  };
}

function teacherDashboard(spaceName: string | undefined): DashboardContent {
  return {
    actionLabel: "进入阅卷",
    description: "聚焦当前授权空间内教师负责的考试、阅卷和成绩查看任务。",
    metrics: [
      { label: "授权空间", value: spaceName ? "1" : "0", trend: "教师视角" },
      { label: "参与考试", value: "3", trend: "我负责的考试" },
      { label: "待阅卷答卷", value: "7", trend: "我负责的待阅卷答卷" },
      { label: "可查看成绩", value: "3", trend: "当前空间成绩单" },
    ],
    rows: [
      { scene: "待阅卷", owner: "教师", status: <StatusBadge tone="warning">待处理</StatusBadge>, next: "处理我负责的简答题评分" },
      { scene: "考试跟进", owner: "教师", status: <StatusBadge tone="info">进行中</StatusBadge>, next: "查看当前空间考试进度" },
      { scene: "成绩查看", owner: "教师", status: <StatusBadge tone="success">可查看</StatusBadge>, next: "查看已发布成绩和导出范围" },
    ],
    statusSubtitle: "当前概览只展示教师在授权空间内可处理的考试业务。",
    statusTitle: "教师工作台状态",
    tableSubtitle: "按当前授权空间整理教师下一步操作。",
    tableTitle: "我的待办",
    title: spaceName ? `${spaceName}教学概览` : "教师工作概览",
  };
}
