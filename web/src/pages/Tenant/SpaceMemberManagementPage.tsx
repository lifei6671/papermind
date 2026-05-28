import { useEffect, useMemo, useState } from "react";
import { Users } from "lucide-react";
import { formatApiErrorMessage } from "../../api/client";
import { spaceApi } from "../../api/spaces";
import type { SpaceMember, SpaceMemberAPI } from "../../api/spaces";
import { useSession } from "../../auth/session-context";
import type { ProfileSpaceAuthorization } from "../../auth/session-context";
import { EmptyState } from "../../components/ui/EmptyState";
import { Panel } from "../../components/ui/Panel";

type SpaceMemberManagementPageProps = {
  api?: SpaceMemberAPI;
  tenantID: number;
};

type AuthorizedSpace = {
  key: string;
  tenantID: number;
  spaceID: number;
  label: string;
};

type MemberResult = {
  spaceID: number;
  members: SpaceMember[];
  errorMessage: string;
};

export function SpaceMemberManagementPage({ api = spaceApi, tenantID }: SpaceMemberManagementPageProps) {
  const { session } = useSession();
  const authorizedSpaces = useMemo(
    () => collectAuthorizedSpaces(session?.profileSpaces ?? [], tenantID),
    [session?.profileSpaces, tenantID],
  );
  const [requestedSpaceID, setRequestedSpaceID] = useState(0);
  const selectedSpaceID = authorizedSpaces.some((space) => space.spaceID === requestedSpaceID)
    ? requestedSpaceID
    : authorizedSpaces[0]?.spaceID ?? 0;
  const [memberResult, setMemberResult] = useState<MemberResult>({
    spaceID: 0,
    members: [],
    errorMessage: "",
  });
  const isLoading = selectedSpaceID > 0 && memberResult.spaceID !== selectedSpaceID;
  const members = isLoading ? [] : memberResult.members;
  const errorMessage = isLoading ? "" : memberResult.errorMessage;

  useEffect(() => {
    let active = true;
    if (!selectedSpaceID) {
      return () => {
        active = false;
      };
    }

    api.listSpaceMembers({ tenantID, spaceID: selectedSpaceID })
      .then((result) => {
        if (active) {
          setMemberResult({
            spaceID: selectedSpaceID,
            members: result.items,
            errorMessage: "",
          });
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setMemberResult({
            spaceID: selectedSpaceID,
            members: [],
            errorMessage: formatApiErrorMessage(error, "空间成员加载失败"),
          });
        }
      });

    return () => {
      active = false;
    };
  }, [api, selectedSpaceID, tenantID]);

  return (
    <section className="page tenant-admin-page space-member-page">
      <div className="toolbar">
        <div>
          <h1>空间成员</h1>
          <p>管理当前账号已授权空间内的成员。</p>
        </div>
      </div>

      <Panel className="space-member-panel" title="授权空间">
        {authorizedSpaces.length === 0 ? (
          <EmptyState icon={Users} title="当前账号没有可管理的空间成员入口" />
        ) : (
          <div className="space-member-layout">
            <div className="space-member-tabs" aria-label="授权空间">
              {authorizedSpaces.map((space) => (
                <button
                  aria-pressed={space.spaceID === selectedSpaceID}
                  className={space.spaceID === selectedSpaceID ? "tenant-action-button tenant-action--open" : "tenant-action-button"}
                  key={space.key}
                  onClick={() => setRequestedSpaceID(space.spaceID)}
                  type="button"
                >
                  {space.label}
                </button>
              ))}
            </div>

            {errorMessage && <p className="tenant-admin-warning">{errorMessage}</p>}
            {isLoading ? (
              <p className="tenant-admin-muted">成员加载中...</p>
            ) : (
              <div className="table-wrap">
                <table className="data-table tenant-admin-table">
                  <thead>
                    <tr>
                      <th>成员</th>
                      <th>用户 ID</th>
                      <th>空间身份</th>
                      <th>状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    {members.length === 0 ? (
                      <tr className="data-table__empty-row">
                        <td className="data-table__empty" colSpan={4}>
                          暂无成员
                        </td>
                      </tr>
                    ) : (
                      members.map((member) => (
                        <tr key={member.id}>
                          <td>{member.name}</td>
                          <td>{member.userID}</td>
                          <td>{memberRoleLabel(member.role)}</td>
                          <td>{member.status === "enabled" ? "启用" : "禁用"}</td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </Panel>
    </section>
  );
}

function collectAuthorizedSpaces(profileSpaces: ProfileSpaceAuthorization[], tenantID: number): AuthorizedSpace[] {
  const seen = new Set<number>();
  return profileSpaces
    .filter((space) => space.tenantID === tenantID && space.role === "space_admin" && space.status === "enabled")
    .filter((space) => {
      if (seen.has(space.spaceID)) {
        return false;
      }
      seen.add(space.spaceID);
      return true;
    })
    .map((space) => ({
      key: `${space.tenantID}-${space.spaceID}`,
      tenantID: space.tenantID,
      spaceID: space.spaceID,
      label: `空间 ${space.spaceID}`,
    }));
}

function memberRoleLabel(role: SpaceMember["role"]) {
  switch (role) {
    case "space_admin":
      return "空间管理员";
    case "teacher":
      return "教师";
    case "student":
      return "学生";
  }
}
