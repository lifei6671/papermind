import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { formatApiErrorMessage } from "../../api/client";
import { spaceApi } from "../../api/spaces";
import type { MemberRole, SpaceMember, SpaceMemberAPI } from "../../api/spaces";
import { useSession } from "../../auth/session-context";
import type { ProfileSpaceAuthorization } from "../../auth/session-context";
import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Pagination } from "../../components/ui/Pagination";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";

const roleLabels: Record<MemberRole, string> = {
  space_admin: "空间管理员",
  teacher: "教师",
  student: "学生",
};

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
  total: number;
};

const memberPageSizeOptions = [5, 10, 20, 50];
const defaultMemberPageSize = 10;

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
  const selectedSpace = authorizedSpaces.find((space) => space.spaceID === selectedSpaceID);
  const [memberResult, setMemberResult] = useState<MemberResult>({
    spaceID: 0,
    members: [],
    errorMessage: "",
    total: 0,
  });
  const [newMemberUserID, setNewMemberUserID] = useState("");
  const [newMemberRole, setNewMemberRole] = useState<MemberRole>("teacher");
  const [actionMessage, setActionMessage] = useState("");
  const [actionError, setActionError] = useState("");
  const [actionKey, setActionKey] = useState("");
  const [refreshKey, setRefreshKey] = useState(0);
  const [memberPage, setMemberPage] = useState(1);
  const [memberPageSize, setMemberPageSize] = useState(defaultMemberPageSize);
  const isLoading = selectedSpaceID > 0 && memberResult.spaceID !== selectedSpaceID;
  const members = isLoading ? [] : memberResult.members;
  const errorMessage = isLoading ? "" : memberResult.errorMessage;
  const memberTotal = isLoading ? 0 : memberResult.total;

  useEffect(() => {
    let active = true;
    if (!selectedSpaceID) {
      return () => {
        active = false;
      };
    }

    api.listSpaceMembers({ tenantID, spaceID: selectedSpaceID, page: memberPage, pageSize: memberPageSize })
      .then((result) => {
        if (active) {
          setMemberResult({
            spaceID: selectedSpaceID,
            members: result.items,
            errorMessage: "",
            total: result.total ?? result.items.length,
          });
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setMemberResult({
            spaceID: selectedSpaceID,
            members: [],
            errorMessage: formatApiErrorMessage(error, "空间成员加载失败"),
            total: 0,
          });
        }
      });

    return () => {
      active = false;
    };
  }, [api, memberPage, memberPageSize, refreshKey, selectedSpaceID, tenantID]);

  useEffect(() => {
    queueMicrotask(() => setMemberPage(1));
  }, [selectedSpaceID]);

  async function handleAddMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const userID = Number(newMemberUserID);
    if (!Number.isInteger(userID) || userID <= 0 || !selectedSpaceID) {
      setActionError("请输入有效的用户 ID");
      setActionMessage("");
      return;
    }

    await runMemberAction("add", async () => {
      // 空间成员只能加入已属于当前租户且启用的用户，最终校验由后端成员接口完成。
      await api.createSpaceMember({ tenantID, spaceID: selectedSpaceID, userID, role: newMemberRole });
      setNewMemberUserID("");
      setNewMemberRole("teacher");
      setActionMessage("成员已添加");
    });
  }

  async function updateMemberRole(member: SpaceMember, role: MemberRole) {
    await runMemberAction(`role:${member.userID}:${role}`, async () => {
      await api.updateSpaceMember({ tenantID, spaceID: selectedSpaceID, userID: member.userID, role });
      setActionMessage("成员身份已更新");
    });
  }

  async function toggleMemberStatus(member: SpaceMember) {
    const nextStatus = member.status === "enabled" ? "disabled" : "enabled";
    await runMemberAction(`status:${member.userID}`, async () => {
      await api.updateSpaceMember({ tenantID, spaceID: selectedSpaceID, userID: member.userID, status: nextStatus });
      setActionMessage(nextStatus === "enabled" ? "成员已启用" : "成员已禁用");
    });
  }

  async function removeMember(member: SpaceMember) {
    await runMemberAction(`remove:${member.userID}`, async () => {
      await api.removeSpaceMember({ tenantID, spaceID: selectedSpaceID, userID: member.userID });
      setActionMessage("成员已移除");
    });
  }

  async function runMemberAction(key: string, action: () => Promise<void>) {
    setActionKey(key);
    setActionError("");
    setActionMessage("");
    try {
      await action();
      setRefreshKey((value) => value + 1);
    } catch (error) {
      setActionError(formatApiErrorMessage(error, "空间成员操作失败"));
    } finally {
      setActionKey("");
    }
  }

  return (
    <section className="page platform-page tenant-admin-page">
      <h1 className="sr-only">空间成员</h1>
      <nav aria-label="空间成员菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/space-members" role="tab" aria-selected="true">
          空间成员
        </a>
      </nav>

      <Panel>
        {authorizedSpaces.length === 0 ? (
          <div className="tenant-admin-warning" role="alert">
            当前账号没有可管理的空间成员入口。
          </div>
        ) : (
          <>
            <div className="tenant-list-toolbar">
              <div className="tenant-list-actions" aria-label="授权空间">
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
              <strong>{selectedSpace ? `${selectedSpace.label}成员` : "空间成员"}</strong>
            </div>

            {errorMessage && <p className="tenant-admin-warning">{errorMessage}</p>}
            {actionError && <p className="tenant-admin-warning" role="alert">{actionError}</p>}
            {actionMessage && <p className="tenant-admin-status">{actionMessage}</p>}
            {isLoading ? (
              <p className="tenant-admin-muted">成员加载中...</p>
            ) : (
              <>
                <form className="tenant-member-form" onSubmit={(event) => void handleAddMember(event)}>
                  <label className="field">
                    <span>用户 ID</span>
                    <input
                      min="1"
                      onChange={(event) => setNewMemberUserID(event.target.value)}
                      placeholder="输入租户用户 ID"
                      type="number"
                      value={newMemberUserID}
                    />
                  </label>
                  <label className="field">
                    <span>空间身份</span>
                    <select onChange={(event) => setNewMemberRole(event.target.value as MemberRole)} value={newMemberRole}>
                      <option value="space_admin">空间管理员</option>
                      <option value="teacher">教师</option>
                      <option value="student">学生</option>
                    </select>
                  </label>
                  <Button disabled={actionKey !== ""} type="submit" variant="secondary">添加成员</Button>
                </form>

                <div className="table-wrap">
                  <table className="data-table tenant-admin-table">
                    <thead>
                      <tr>
                        <th scope="col">成员</th>
                        <th scope="col">角色</th>
                        <th scope="col">状态</th>
                        <th scope="col">操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {members.length === 0 ? <EmptyTableRow colSpan={4} label="暂无成员" /> : (
                        members.map((member) => (
                          <tr key={member.id}>
                            <td>{member.name}（ID {member.userID}）</td>
                            <td>
                              <select
                                aria-label={`修改 ${member.name} 的空间身份`}
                                disabled={actionKey !== ""}
                                onChange={(event) => void updateMemberRole(member, event.target.value as MemberRole)}
                                value={member.role}
                              >
                                <option value="space_admin">{roleLabels.space_admin}</option>
                                <option value="teacher">{roleLabels.teacher}</option>
                                <option value="student">{roleLabels.student}</option>
                              </select>
                            </td>
                            <td>
                              <StatusBadge tone={member.status === "enabled" ? "success" : "warning"}>
                                {member.status === "enabled" ? "启用" : "禁用"}
                              </StatusBadge>
                            </td>
                            <td>
                              <div className="tenant-actions">
                                <Button
                                  disabled={actionKey !== ""}
                                  onClick={() => void toggleMemberStatus(member)}
                                  type="button"
                                  variant={member.status === "enabled" ? "actionClose" : "actionOpen"}
                                >
                                  {member.status === "enabled" ? "禁用" : "启用"}
                                </Button>
                                <Button
                                  disabled={actionKey !== ""}
                                  onClick={() => void removeMember(member)}
                                  type="button"
                                  variant="actionClose"
                                >
                                  移除
                                </Button>
                              </div>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
                <Pagination
                  onPageChange={setMemberPage}
                  onPageSizeChange={(nextPageSize) => {
                    setMemberPage(1);
                    setMemberPageSize(nextPageSize);
                  }}
                  page={memberPage}
                  pageSize={memberPageSize}
                  pageSizeOptions={memberPageSizeOptions}
                  total={memberTotal}
                />
              </>
            )}
          </>
        )}
      </Panel>
    </section>
  );
}

function collectAuthorizedSpaces(profileSpaces: ProfileSpaceAuthorization[], tenantID: number): AuthorizedSpace[] {
  const seen = new Set<number>();
  return profileSpaces
    .filter((space) =>
      space.tenantID === tenantID &&
      space.spaceID > 0 &&
      (space.role === "tenant_admin" || space.role === "space_admin") &&
      space.status === "enabled",
    )
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
      label: space.spaceName || `空间 ${space.spaceID}`,
    }));
}
