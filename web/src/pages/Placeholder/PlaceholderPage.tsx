import { EmptyState } from "../../components/ui/EmptyState";
import { Panel } from "../../components/ui/Panel";
import { SectionHeader } from "../../components/ui/SectionHeader";

type PlaceholderPageProps = {
  title: string;
  description: string;
};

export function PlaceholderPage({ title, description }: PlaceholderPageProps) {
  return (
    <section className="page">
      <SectionHeader description={description} title={title} />
      <Panel title={`${title}页面`} subtitle="当前阶段只迁移 UI 骨架，业务接口按 P9 清单逐项接入。">
        <EmptyState title="页面骨架已预留" />
        <div className="empty-state-actions">
          <p>这里会沿用 axiom-ui 的面板、表格、状态标签和表单密度来实现 Papermind 管理端。</p>
          <button className="secondary-button" type="button">查看执行清单</button>
        </div>
      </Panel>
    </section>
  );
}
