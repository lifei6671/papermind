import { Button } from "../../components/ui/Button";
import { EmptyState } from "../../components/ui/EmptyState";
import { Panel } from "../../components/ui/Panel";

type PlaceholderPageProps = {
  title: string;
  description: string;
};

export function PlaceholderPage({ title, description }: PlaceholderPageProps) {
  return (
    <section className="page platform-page">
      <nav aria-label={`${title}菜单`} className="platform-tabbar" role="tablist">
        <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
          {title}
        </span>
      </nav>
      <Panel>
        <EmptyState title="页面骨架已预留" />
        <div className="empty-state-actions">
          <p>{description}</p>
          <Button variant="secondary" type="button">查看执行清单</Button>
        </div>
      </Panel>
    </section>
  );
}
