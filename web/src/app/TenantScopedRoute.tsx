import type { ReactElement } from "react";
import { useSearchParams } from "react-router-dom";
import type { SessionUser } from "../auth/session-context";
import { EmptyState } from "../components/ui/EmptyState";
import { Panel } from "../components/ui/Panel";

type TenantScopedRouteProps = {
  render: (tenantID: number) => ReactElement;
  title: string;
  user?: SessionUser | null;
};

export function TenantScopedRoute({ render, title, user }: TenantScopedRouteProps) {
  const tenantID = useTargetTenantID(user);
  if (!tenantID) {
    return (
      <section className="page platform-page tenant-admin-page">
        <nav aria-label={`${title}菜单`} className="platform-tabbar" role="tablist">
          <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
            {title}
          </span>
        </nav>
        <Panel>
          <EmptyState title="请选择目标租户" />
          <div className="empty-state-actions">
            <p>平台管理员需要从租户管理进入具体租户，或在地址中携带 tenant_id。</p>
          </div>
        </Panel>
      </section>
    );
  }
  return render(tenantID);
}

function useTargetTenantID(user?: SessionUser | null) {
  const [searchParams] = useSearchParams();
  return positiveID(searchParams.get("tenant_id")) ?? positiveID(user?.tenantID);
}

function positiveID(value?: string | number | null) {
  if (value === undefined || value === null || value === "") {
    return undefined;
  }
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}
