import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowRight, Building2, DoorOpen, GraduationCap, School } from "lucide-react";
import { authApi } from "../../api/auth";
import type { AuthAPI, ProfileSpaceMembership } from "../../api/auth";
import { formatApiErrorMessage } from "../../api/client";
import { useSession } from "../../auth/session-context";
import { Button } from "../../components/ui/Button";
import { EmptyState } from "../../components/ui/EmptyState";
import { Panel } from "../../components/ui/Panel";

type TenantEntryPageProps = {
  api?: AuthAPI;
};

type TenantEntryView = {
  kind: "tenant" | "space" | "exam";
  key: string;
  membership: ProfileSpaceMembership;
  role: ProfileSpaceMembership["role"];
  subtitle: string;
  title: string;
};

export function TenantEntryPage({ api = authApi }: TenantEntryPageProps) {
  const navigate = useNavigate();
  const { session, signIn } = useSession();
  const [spaces, setSpaces] = useState<ProfileSpaceMembership[]>([]);
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [selectingKey, setSelectingKey] = useState("");

  useEffect(() => {
    let alive = true;
    api.listProfileSpaces()
      .then((result) => {
        if (alive) {
          setSpaces(result.items);
          setError("");
        }
      })
      .catch((err) => {
        if (alive) {
          setError(formatApiErrorMessage(err, "读取可进入空间失败"));
        }
      })
      .finally(() => {
        if (alive) {
          setIsLoading(false);
        }
      });
    return () => {
      alive = false;
    };
  }, [api]);

  const entries = buildEntryViews(spaces);

  async function enterEntry(entry: TenantEntryView) {
    const key = entry.key;
    setSelectingKey(key);
    setError("");
    try {
      // 租户管理员只绑定租户上下文；空间角色才绑定具体空间。
      const nextSession = await api.selectTenantSpace(entry.kind === "tenant"
        ? { tenantID: entry.membership.tenantID }
        : { tenantID: entry.membership.tenantID, spaceID: entry.membership.spaceID });
      const profileSpaces = await api.listProfileSpaces();
      const selectedSpaceID = entry.kind === "tenant" ? undefined : entry.membership.spaceID;
      signIn({ ...nextSession, selectedSpaceID, profileSpaces: profileSpaces.items });
      navigate(entry.kind === "exam" ? "/exam-entry" : "/", { replace: true });
    } catch (err) {
      setError(formatApiErrorMessage(err, "进入空间失败"));
    } finally {
      setSelectingKey("");
    }
  }

  if (!session) {
    return null;
  }

  return (
    <main className="tenant-entry-page">
      <section className="tenant-entry-page__header">
        <div>
          <span className="tenant-entry-page__eyebrow">PaperMind</span>
          <h1>选择入口</h1>
        </div>
        <div className="tenant-entry-page__user">
          <Building2 aria-hidden="true" size={18} />
          <span>{session.user.displayName}</span>
        </div>
      </section>

      {error && <div className="tenant-entry-page__error" role="alert">{error}</div>}

      <Panel className="tenant-entry-page__panel">
        {isLoading ? (
          <EmptyState title="正在读取空间" />
        ) : entries.length === 0 ? (
          <EmptyState title="暂无可进入入口" />
        ) : (
          <div className="tenant-entry-grid">
            {entries.map((entry) => {
              return (
                <button
                  className="tenant-entry-card"
                  disabled={selectingKey !== ""}
                  key={entry.key}
                  onClick={() => void enterEntry(entry)}
                  type="button"
                >
                  <span className="tenant-entry-card__icon">
                    {entryIcon(entry.kind)}
                  </span>
                  <span className="tenant-entry-card__body">
                    <strong>{entry.title}</strong>
                    <span>{entry.subtitle}</span>
                  </span>
                  <span className="tenant-entry-card__role">{roleLabel(entry.role)}</span>
                  <ArrowRight aria-hidden="true" size={18} />
                </button>
              );
            })}
          </div>
        )}
      </Panel>

      <div className="tenant-entry-page__actions">
        <Button onClick={() => navigate("/login", { replace: true })} type="button" variant="secondary">
          返回登录
        </Button>
      </div>
    </main>
  );
}

function buildEntryViews(spaces: ProfileSpaceMembership[]): TenantEntryView[] {
  const tenantEntries = new Map<number, TenantEntryView>();
  const scopedEntries: TenantEntryView[] = [];

  spaces
    .filter((space) => space.status === "enabled")
    .forEach((space) => {
      if (space.role === "tenant_admin") {
        if (!tenantEntries.has(space.tenantID)) {
          tenantEntries.set(space.tenantID, {
            kind: "tenant",
            key: `${space.tenantID}:tenant_admin`,
            membership: { ...space, spaceID: 0 },
            role: "tenant_admin",
            subtitle: "租户管理后台",
            title: space.tenantName || `租户 ${space.tenantID}`,
          });
        }
        return;
      }

      const kind = space.role === "student" ? "exam" : "space";
      scopedEntries.push({
        kind,
        key: `${space.tenantID}:${space.spaceID}:${space.role}`,
        membership: space,
        role: space.role,
        subtitle: `${space.tenantName || `租户 ${space.tenantID}`} · ${entrySubtitle(space.role)}`,
        title: space.spaceName || `空间 ${space.spaceID}`,
      });
    });

  return [...tenantEntries.values(), ...scopedEntries];
}

function entrySubtitle(role: ProfileSpaceMembership["role"]) {
  switch (role) {
    case "tenant_admin":
      return "租户管理后台";
    case "space_admin":
      return "空间管理入口";
    case "teacher":
      return "教学业务入口";
    case "student":
      return "考试入口";
  }
}

function entryIcon(kind: TenantEntryView["kind"]) {
  switch (kind) {
    case "tenant":
      return <DoorOpen aria-hidden="true" size={22} />;
    case "space":
      return <School aria-hidden="true" size={22} />;
    case "exam":
      return <GraduationCap aria-hidden="true" size={22} />;
  }
}

function roleLabel(role: ProfileSpaceMembership["role"]) {
  switch (role) {
    case "tenant_admin":
      return "租户管理员";
    case "space_admin":
      return "空间管理员";
    case "teacher":
      return "教师";
    case "student":
      return "学生";
  }
}
