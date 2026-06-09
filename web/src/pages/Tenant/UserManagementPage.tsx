import { Button } from "../../components/ui/Button";
import Descriptions from "antd/es/descriptions";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { useFeedback } from "../../app/feedback-context";
import { Pagination } from "../../components/ui/Pagination";
import { Search, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Panel } from "../../components/ui/Panel";
import { PlatformDrawer } from "../../components/ui/PlatformDrawer";
import { PlatformModal } from "../../components/ui/PlatformModal";
import { RefreshIcon } from "../../components/ui/RefreshIcon";
import { withRefreshFeedback } from "../../components/ui/refreshFeedback";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { spaceApi as defaultSpaceApi } from "../../api/spaces";
import type { MemberRole, SpaceManagementAPI, SpaceMemberAPI, SpaceRow } from "../../api/spaces";
import { formatApiErrorMessage } from "../../api/client";
import { userApi } from "../../api/users";
import type { TenantUserRow, UserManagementAPI, UserRole } from "../../api/users";

const roleLabels: Record<UserRole, string> = {
  tenant_admin: "租户管理员",
  teacher: "教师",
  student: "学生",
};
const userPageSize = 5;


type UserManagementPageProps = {
  api?: UserManagementAPI;
  spaceApi?: Pick<SpaceManagementAPI, "listSpaces"> & Pick<SpaceMemberAPI, "createSpaceMember" | "removeSpaceMember" | "updateSpaceMember">;
  tenantID?: number;
  actorID?: number;
};

export function UserManagementPage({
  api = userApi,
  spaceApi = defaultSpaceApi,
  tenantID,
  actorID,
}: UserManagementPageProps) {
  const [users, setUsers] = useState<TenantUserRow[]>([]);
  const [spaces, setSpaces] = useState<SpaceRow[]>([]);
  const [name, setName] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<UserRole>("student");
  const [forcePasswordChange, setForcePasswordChange] = useState(true);
  const [selectedCreateSpaceIDs, setSelectedCreateSpaceIDs] = useState<number[]>([]);
  const [detailTarget, setDetailTarget] = useState<TenantUserRow | null>(null);
  const [editTarget, setEditTarget] = useState<TenantUserRow | null>(null);
  const [selectedEditSpaceIDs, setSelectedEditSpaceIDs] = useState<number[]>([]);
  const [disableTarget, setDisableTarget] = useState<TenantUserRow | null>(null);
  const [isCreateDrawerOpen, setIsCreateDrawerOpen] = useState(false);
  const [userPage, setUserPage] = useState(1);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [loadError, setLoadError] = useState("");
  const [isUserListRefreshing, setIsUserListRefreshing] = useState(false);
  const hasTenantContext = isPositiveInteger(tenantID);
  const { showError } = useFeedback();

  useEffect(() => {
    if (!isPositiveInteger(tenantID)) {
      return;
    }

    let ignore = false;
    const currentTenantID = tenantID;

    Promise.all([api.listUsers(currentTenantID), spaceApi.listSpaces(currentTenantID)])
      .then(([userData, spaceData]) => {
        if (!ignore) {
          setUsers(userData.items);
          setSpaces(spaceData.items);
          setLoadError("");
        }
      })
      .catch(() => {
        if (!ignore) {
          setLoadError("用户或空间列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, spaceApi, tenantID]);

  const filteredUsers = users.filter((item) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    // 用户列表搜索只匹配当前表格可见字段，便于租户管理员按姓名、账号、角色或状态定位。
    return [
      item.name,
      item.username,
      item.avatarFileName,
      roleLabels[item.role],
      userSpaceAssignmentLabel(item, spaces),
      item.status === "enabled" ? "启用" : "禁用",
    ].some((value) => value.toLowerCase().includes(keyword));
  });
  const totalUserPages = Math.max(1, Math.ceil(filteredUsers.length / userPageSize));
  const currentUserPage = Math.min(userPage, totalUserPages);
  const pagedUsers = filteredUsers.slice(
    (currentUserPage - 1) * userPageSize,
    currentUserPage * userPageSize,
  );

  async function handleCreateUser(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!isPositiveInteger(tenantID)) {
      setLoadError("当前账号没有租户上下文，请先登录租户账号后再创建用户");
      return;
    }

    try {
      const nextUser = await api.createUser({
        tenantID,
        name,
        username,
        password,
        role,
        avatarFileName: "未上传",
        forcePasswordChange,
      });

      await syncUserSpaceAssignments(nextUser, role, selectedCreateSpaceIDs);
      setUsers((items) => [nextUser, ...items]);
      setUserPage(1);
      closeCreateDrawer();
    } catch (error) {
      showError(formatApiErrorMessage(error, "创建用户失败"));
    }
  }

  function openCreateDrawer() {
    setIsCreateDrawerOpen(true);
  }

  function closeCreateDrawer() {
    setName("");
    setUsername("");
    setPassword("");
    setRole("student");
    setForcePasswordChange(true);
    setSelectedCreateSpaceIDs([]);
    setIsCreateDrawerOpen(false);
  }

  function openEditDrawer(user: TenantUserRow) {
    setEditTarget(user);
    setSelectedEditSpaceIDs(assignedSpaceIDs(user, spaces));
  }

  function closeEditDrawer() {
    setEditTarget(null);
    setSelectedEditSpaceIDs([]);
  }

  async function handleEditUser(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editTarget) {
      return;
    }

    if (!isPositiveInteger(tenantID)) {
      setLoadError("当前账号没有租户上下文，请先登录租户账号后再编辑用户");
      return;
    }

    try {
      await syncUserSpaceAssignments(editTarget, editTarget.role, selectedEditSpaceIDs);
      setUsers((items) => items.map((item) => (item.id === editTarget.id ? editTarget : item)));
      closeEditDrawer();
    } catch (error) {
      showError(formatApiErrorMessage(error, "保存用户失败"));
      try {
        await reloadUsersAndSpaces(tenantID);
      } catch {
        setLoadError("用户或空间列表加载失败");
      }
    }
  }

  async function reloadUsersAndSpaces(currentTenantID: number) {
    const [userData, spaceData] = await Promise.all([
      api.listUsers(currentTenantID),
      spaceApi.listSpaces(currentTenantID),
    ]);
    setUsers(userData.items);
    setSpaces(spaceData.items);
    setLoadError("");
  }

  async function syncUserSpaceAssignments(user: TenantUserRow, nextRole: UserRole, selectedSpaceIDs: number[]) {
    if (!isPositiveInteger(tenantID)) {
      return;
    }

    const nextMemberRole = memberRoleForUserRole(nextRole);
    const selectedIDs = new Set(nextMemberRole ? selectedSpaceIDs : []);
    const operations = spaces.map(async (space) => {
      const currentMember = space.members.find((member) => member.userID === user.id);
      if (currentMember?.role === "space_admin") {
        return;
      }
      const shouldAssign = selectedIDs.has(space.id);

      if (shouldAssign && !currentMember && nextMemberRole) {
        await spaceApi.createSpaceMember({ tenantID, spaceID: space.id, userID: user.id, role: nextMemberRole });
      } else if (shouldAssign && currentMember && nextMemberRole && (currentMember.role !== nextMemberRole || currentMember.status !== "enabled")) {
        await spaceApi.updateSpaceMember({
          tenantID,
          spaceID: space.id,
          userID: user.id,
          role: nextMemberRole,
          status: "enabled",
        });
      } else if (!shouldAssign && currentMember) {
        await spaceApi.removeSpaceMember({ tenantID, spaceID: space.id, userID: user.id });
      }
    });

    await Promise.all(operations);
    setSpaces((items) =>
      items.map((space) => {
        const shouldAssign = selectedIDs.has(space.id);
        const existingMember = space.members.find((member) => member.userID === user.id);
        if (existingMember?.role === "space_admin") {
          return space;
        }
        const membersWithoutUser = space.members.filter((member) => member.userID !== user.id);
        if (!shouldAssign || !nextMemberRole) {
          return { ...space, members: membersWithoutUser };
        }

        return {
          ...space,
          members: [
            ...membersWithoutUser,
            {
              id: existingMember?.id ?? user.id,
              userID: user.id,
              name: user.name,
              role: nextMemberRole,
              status: "enabled",
            },
          ],
        };
      }),
    );
  }

  async function confirmDisableUser() {
    if (!disableTarget) {
      return;
    }

    if (!isPositiveInteger(tenantID)) {
      setLoadError("当前账号没有租户上下文，请先登录租户账号后再禁用用户");
      return;
    }

    if (!isPositiveInteger(actorID)) {
      setLoadError("当前账号没有操作人上下文，请重新登录后再禁用用户");
      return;
    }
    if (disableTarget.id === actorID) {
      showError("不能禁用当前登录用户");
      setDisableTarget(null);
      return;
    }

    try {
      const disabledUser = await api.disableUser({
        tenantID,
        actorID,
        userID: disableTarget.id,
      });

      setUsers((items) =>
        items.map((item) =>
          item.id === disableTarget.id ? disabledUser : item,
        ),
      );
      setDisableTarget(null);
    } catch (error) {
      showError(formatApiErrorMessage(error, "禁用用户失败"));
    }
  }

  async function enableUser(target: TenantUserRow) {
    if (!isPositiveInteger(tenantID)) {
      setLoadError("当前账号没有租户上下文，请先登录租户账号后再启用用户");
      return;
    }

    if (!isPositiveInteger(actorID)) {
      setLoadError("当前账号没有操作人上下文，请重新登录后再启用用户");
      return;
    }

    const enabledUser = await api.enableUser({
      tenantID,
      actorID,
      userID: target.id,
    });

    setUsers((items) => items.map((item) => (item.id === target.id ? enabledUser : item)));
  }

  function handleSearchUsers() {
    setUserPage(1);
    setAppliedSearchQuery(searchQuery);
  }

  async function handleRefreshUsers() {
    if (!isPositiveInteger(tenantID)) {
      return;
    }

    setSearchQuery("");
    setAppliedSearchQuery("");
    setUserPage(1);
    setIsUserListRefreshing(true);
    try {
      const [userData, spaceData] = await withRefreshFeedback(
        Promise.all([api.listUsers(tenantID), spaceApi.listSpaces(tenantID)]),
      );
      setUsers(userData.items);
      setSpaces(spaceData.items);
      setLoadError("");
    } catch {
      setLoadError("用户或空间列表加载失败");
    } finally {
      setIsUserListRefreshing(false);
    }
  }

  if (!hasTenantContext) {
    return (
      <section className="page platform-page tenant-admin-page">
        <nav aria-label="用户管理菜单" className="platform-tabbar" role="tablist">
          <a className="platform-tab platform-tab--active" href="/users" role="tab" aria-selected="true">
            用户管理
          </a>
        </nav>

        <Panel>
          <div className="tenant-admin-warning" role="alert">
            当前账号没有租户上下文，请先登录租户账号后再进入用户管理。
          </div>
        </Panel>
      </section>
    );
  }

  return (
    <section className="page platform-page tenant-admin-page">
      <nav aria-label="用户管理菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/users" role="tab" aria-selected="true">
          用户管理
        </a>
      </nav>

      <Panel>
          <div className="tenant-list-toolbar">
            <div className="tenant-list-actions" aria-label="用户操作区">
              <Button
              variant="toolbarPrimary"
              onClick={openCreateDrawer}
              type="button"
            >
              创建用户
            </Button>
            </div>
            <div className="tenant-search-actions">
              <label className="tenant-search-field">
                <span className="sr-only">搜索用户</span>
                <input
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="输入姓名、账号、角色或状态"
                  value={searchQuery}
                />
              </label>
              <Button aria-label="搜索" variant="icon" onClick={handleSearchUsers} type="button">
                <Search aria-hidden="true" size={16} />
              </Button>
              <Button
                aria-label="刷新用户列表"
                variant="icon"
                disabled={isUserListRefreshing}
                onClick={() => void handleRefreshUsers()}
                type="button"
              >
                <RefreshIcon active={isUserListRefreshing} />
              </Button>
            </div>
          </div>
        {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">用户</th>
                <th scope="col">账号</th>
                <th scope="col">头像</th>
                <th scope="col">角色</th>
                <th scope="col">空间分配</th>
                <th scope="col">状态</th>
                <th scope="col">操作</th>
              </tr>
            </thead>
            <tbody>
              {filteredUsers.length === 0 && <EmptyTableRow colSpan={7} />}
              {pagedUsers.map((item) => (
                <tr key={item.id}>
                  <td>{item.name}</td>
                  <td>{item.username}</td>
                  <td>{item.avatarFileName}</td>
                  <td>{roleLabels[item.role]}</td>
                  <td>{userSpaceAssignmentLabel(item, spaces)}</td>
                  <td>
                    <StatusBadge tone={item.status === "enabled" ? "success" : "info"}>
                      {item.status === "enabled" ? "启用" : "禁用"}
                    </StatusBadge>
                  </td>
                  <td>
                    <Button
                      variant="actionOpen"
                      onClick={() => setDetailTarget(item)}
                      type="button"
                    >
                      查看详情
                    </Button>
                    <Button
                      variant="actionEdit"
                      onClick={() => openEditDrawer(item)}
                      type="button"
                    >
                      编辑用户
                    </Button>
                    {item.status === "enabled" ? (
                      <Button
                        disabled={item.id === actorID}
                        title={item.id === actorID ? "不能禁用当前登录用户" : undefined}
                        variant="actionClose"
                        onClick={() => setDisableTarget(item)}
                        type="button"
                      >
                        禁用用户
                      </Button>
                    ) : (
                      <Button
                        variant="actionOpen"
                        onClick={() => void enableUser(item)}
                        type="button"
                      >
                        启用用户
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <Pagination
          page={currentUserPage}
          pageSize={userPageSize}
          total={filteredUsers.length}
          onPageChange={setUserPage}
        />
      </Panel>

      <PlatformDrawer ariaLabel="创建用户抽屉" onClose={closeCreateDrawer} open={isCreateDrawerOpen}>
        <header className="tenant-resource-drawer__head tenant-resource-drawer__head--inline">
          <div className="tenant-resource-drawer__title">
            <span>新建用户</span>
            <h2>创建用户</h2>
          </div>
          <button
            aria-label="关闭抽屉"
            className="tenant-resource-drawer__icon"
            onClick={closeCreateDrawer}
            type="button"
          >
            <X aria-hidden="true" size={16} />
          </button>
        </header>

        <form className="platform-form tenant-user-drawer-form" onSubmit={handleCreateUser}>
          <label className="field">
            <RequiredLabel>姓名</RequiredLabel>
            <input aria-label="姓名" onChange={(event) => setName(event.target.value)} required value={name} />
          </label>
          <label className="field">
            <RequiredLabel>账号</RequiredLabel>
            <input aria-label="账号" onChange={(event) => setUsername(event.target.value)} required value={username} />
          </label>
          <label className="field">
            <RequiredLabel>初始密码</RequiredLabel>
            <input
              aria-label="初始密码"
              minLength={8}
              onChange={(event) => setPassword(event.target.value)}
              required
              type="password"
              value={password}
            />
          </label>
          <label className="platform-check">
            <input
              checked={forcePasswordChange}
              onChange={(event) => setForcePasswordChange(event.target.checked)}
              type="checkbox"
            />
            <span>首次登录必须修改密码</span>
          </label>
          <label className="field">
            <RequiredLabel>角色</RequiredLabel>
            <select aria-label="角色" onChange={(event) => setRole(event.target.value as UserRole)} required value={role}>
              <option value="tenant_admin">租户管理员角色</option>
              <option value="teacher">教师角色</option>
              <option value="student">学生角色</option>
            </select>
          </label>
          <SpaceAssignmentField
            role={role}
            selectedSpaceIDs={selectedCreateSpaceIDs}
            spaces={spaces}
            onChange={setSelectedCreateSpaceIDs}
          />
          <div className="platform-dialog__actions">
            <Button variant="secondary" onClick={closeCreateDrawer} type="button">
              取消
            </Button>
            <Button variant="primary" type="submit">
              确认创建
            </Button>
          </div>
        </form>
      </PlatformDrawer>

      {editTarget && (
        <PlatformDrawer ariaLabel="编辑用户抽屉" onClose={closeEditDrawer} open={editTarget !== null}>
          <header className="tenant-resource-drawer__head tenant-resource-drawer__head--inline">
            <div className="tenant-resource-drawer__title">
              <span>编辑用户</span>
              <h2>{editTarget.name}</h2>
            </div>
            <button
              aria-label="关闭抽屉"
              className="tenant-resource-drawer__icon"
              onClick={closeEditDrawer}
              type="button"
            >
              <X aria-hidden="true" size={16} />
            </button>
          </header>

          <form className="platform-form tenant-user-drawer-form" onSubmit={handleEditUser}>
            <label className="field">
              <span>姓名</span>
              <input aria-label="姓名" readOnly value={editTarget.name} />
            </label>
            <label className="field">
              <span>账号</span>
              <input aria-label="账号" readOnly value={editTarget.username} />
            </label>
            <label className="field">
              <span>角色</span>
              <input aria-label="角色" readOnly value={roleLabels[editTarget.role]} />
            </label>
            <SpaceAssignmentField
              lockedSpaceIDs={protectedAssignedSpaceIDs(editTarget, spaces)}
              role={editTarget.role}
              selectedSpaceIDs={selectedEditSpaceIDs}
              spaces={spaces}
              onChange={setSelectedEditSpaceIDs}
            />
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={closeEditDrawer} type="button">
                取消
              </Button>
              <Button variant="primary" type="submit">
                保存用户
              </Button>
            </div>
          </form>
        </PlatformDrawer>
      )}

      {detailTarget && (
        <PlatformDrawer ariaLabel="用户详情" onClose={() => setDetailTarget(null)} open={detailTarget !== null}>
          <header className="tenant-resource-drawer__head tenant-resource-drawer__head--inline">
            <div className="tenant-resource-drawer__title">
              <span>用户详情</span>
              <h2>{detailTarget.name}</h2>
            </div>
            <button
              aria-label="关闭抽屉"
              className="tenant-resource-drawer__icon"
              onClick={() => setDetailTarget(null)}
              type="button"
            >
              <X aria-hidden="true" size={16} />
            </button>
          </header>

          <Descriptions
            bordered
            className="tenant-user-detail-descriptions"
            column={1}
            size="small"
          >
            <Descriptions.Item label="账号">{detailTarget.username}</Descriptions.Item>
            <Descriptions.Item label="角色">{roleLabels[detailTarget.role]}</Descriptions.Item>
            <Descriptions.Item label="空间分配">
              {userSpaceAssignmentLabel(detailTarget, spaces)}
            </Descriptions.Item>
            <Descriptions.Item label="状态">
              {detailTarget.status === "enabled" ? "启用" : "禁用"}
            </Descriptions.Item>
          </Descriptions>
        </PlatformDrawer>
      )}

      {disableTarget && (
        <PlatformModal open={disableTarget !== null} onClose={() => setDisableTarget(null)} title="禁用用户提示">
            <h2>禁用 {disableTarget.name}</h2>
            <p className="tenant-admin-warning">
              将失去登录能力，阅卷和考试发布权限会被收回，已提交答卷和历史成绩不受影响。
            </p>
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={() => setDisableTarget(null)} type="button">
                取消
              </Button>
              <Button variant="primary" onClick={confirmDisableUser} type="button">
                确认禁用
              </Button>
            </div>
        </PlatformModal>
      )}
    </section>
  );
}

function RequiredLabel({ children }: { children: string }) {
  return (
    <span className="field-label">
      {children}
      <span aria-hidden="true" className="required-marker">*</span>
    </span>
  );
}

type SpaceAssignmentFieldProps = {
  lockedSpaceIDs?: number[];
  onChange: (spaceIDs: number[]) => void;
  role: UserRole;
  selectedSpaceIDs: number[];
  spaces: SpaceRow[];
};

function SpaceAssignmentField({ lockedSpaceIDs = [], onChange, role, selectedSpaceIDs, spaces }: SpaceAssignmentFieldProps) {
  const memberRole = memberRoleForUserRole(role);

  if (!memberRole) {
    return (
      <div className="tenant-user-space-field">
        <span className="field-label">分配空间</span>
        <p>租户管理员不需要分配到具体空间。</p>
      </div>
    );
  }

  const selectedIDs = new Set(selectedSpaceIDs);
  const lockedIDs = new Set(lockedSpaceIDs);

  return (
    <fieldset className="tenant-user-space-field">
      <legend>分配空间</legend>
      {spaces.length === 0 ? (
        <p>暂无可分配空间</p>
      ) : (
        <div className="tenant-user-space-options">
          {spaces.map((space) => (
            <label className="tenant-user-space-option" key={space.id}>
              <input
                disabled={lockedIDs.has(space.id)}
                checked={selectedIDs.has(space.id)}
                onChange={(event) => {
                  if (lockedIDs.has(space.id)) {
                    return;
                  }
                  if (event.target.checked) {
                    onChange([...selectedSpaceIDs, space.id]);
                    return;
                  }
                  onChange(selectedSpaceIDs.filter((spaceID) => spaceID !== space.id));
                }}
                type="checkbox"
              />
              <span>{space.name}</span>
            </label>
          ))}
        </div>
      )}
    </fieldset>
  );
}

function isPositiveInteger(value: number | undefined): value is number {
  return Number.isInteger(value) && Number(value) > 0;
}

function userSpaceAssignmentLabel(user: TenantUserRow, spaces: SpaceRow[]) {
  if (user.role === "tenant_admin") {
    return "不适用";
  }

  const assignedSpaceCount = assignedSpaceIDs(user, spaces).length;

  return assignedSpaceCount > 0 ? `已加入 ${assignedSpaceCount} 个空间` : "暂未分配空间";
}

function assignedSpaceIDs(user: TenantUserRow, spaces: SpaceRow[]) {
  if (user.role === "tenant_admin") {
    return [];
  }

  return spaces
    .filter((space) =>
      space.members.some((member) =>
        member.userID === user.id &&
        member.status === "enabled" &&
        memberRoleMatchesUserRole(user.role, member.role),
      ),
    )
    .map((space) => space.id);
}

function protectedAssignedSpaceIDs(user: TenantUserRow, spaces: SpaceRow[]) {
  return spaces
    .filter((space) =>
      space.members.some((member) =>
        member.userID === user.id &&
        member.status === "enabled" &&
        member.role === "space_admin",
      ),
    )
    .map((space) => space.id);
}

function memberRoleMatchesUserRole(userRole: UserRole, memberRole: MemberRole) {
  if (userRole === "teacher") {
    return memberRole === "teacher" || memberRole === "space_admin";
  }
  return memberRole === userRole;
}

function memberRoleForUserRole(role: UserRole): MemberRole | null {
  if (role === "tenant_admin") {
    return null;
  }

  return role;
}
