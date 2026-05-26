import { Button } from "../../components/ui/Button";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { Home, LogOut, Settings } from "lucide-react";
import { useSession } from "../../auth/session-context";
import type { AdminRoute, AdminRouteGroup } from "../../app/routes";
import "./AdminShell.css";

type AdminShellProps = {
  routes: AdminRoute[];
};

const groupLabels: Record<Exclude<AdminRouteGroup, "hidden">, string> = {
  platform: "平台运营",
  tenant: "租户空间",
  exam: "考试业务",
};

export function AdminShell({ routes }: AdminShellProps) {
  const navigate = useNavigate();
  const { signOut } = useSession();
  const visibleGroups = Object.keys(groupLabels) as Array<Exclude<AdminRouteGroup, "hidden">>;

  const renderRouteLink = (route: AdminRoute) => {
    const Icon = route.icon;

    return (
      <NavLink
        className={({ isActive }) => (isActive ? "menu-link menu-link--active" : "menu-link")}
        end={route.path === "/"}
        key={route.path}
        to={route.path}
      >
        <Icon aria-hidden="true" size={18} strokeWidth={1.8} />
        <span>{route.label}</span>
        {route.badge && <em>{route.badge}</em>}
      </NavLink>
    );
  };

  return (
    <div className="admin-shell">
      <aside className="sidebar" aria-label="后台导航">
        <div className="brand-rail">
          <div className="brand-mark" aria-hidden="true">
            P
          </div>
          <span className="brand-word">Papermind</span>
        </div>

        <div className="menu-panel">
          {visibleGroups.map((group) => {
            const groupRoutes = routes.filter((route) => route.group === group);

            return (
              <div className="menu-panel__section" key={group}>
                <span className="eyebrow">{groupLabels[group]}</span>
                <nav className="menu-panel__nav" aria-label={groupLabels[group]}>
                  {groupRoutes.map(renderRouteLink)}
                </nav>
              </div>
            );
          })}

          <div className="menu-panel__note">
            <div>
              <span>平台管理员</span>
              <p>本地管理端 UI 骨架预览</p>
            </div>
            <NavLink
              aria-label="个人设置"
              className={({ isActive }) =>
                isActive ? "menu-panel__settings menu-panel__settings--active" : "menu-panel__settings"
              }
              to="/settings/profile"
            >
              <Settings aria-hidden="true" size={15} strokeWidth={1.8} />
            </NavLink>
          </div>

          <Button className="sidebar__home" onClick={() => navigate("/")} type="button">
            <Home aria-hidden="true" size={15} />
            回到概览
          </Button>
          <Button
            className="sidebar__logout"
            onClick={() => {
              signOut();
              navigate("/login", { replace: true });
            }}
            type="button"
          >
            <LogOut aria-hidden="true" size={15} />
            退出登录
          </Button>
        </div>
      </aside>

      <div className="workspace">
        <main className="workspace__content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
