import AutoComplete from "antd/es/auto-complete";
import MDEditor from "@uiw/react-md-editor/nohighlight";
import "@uiw/react-md-editor/markdown-editor.css";
import "@uiw/react-markdown-preview/markdown.css";
import "katex/dist/katex.min.css";
import { localizeMarkdownEditorCommand } from "../../components/markdown/markdownEditorCommands";
import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Pagination } from "../../components/ui/Pagination";
import { Maximize2, Minimize2, Plus, Search } from "lucide-react";
import { useEffect, useState } from "react";
import rehypeKatex from "rehype-katex";
import remarkMath from "remark-math";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { PlatformDrawer } from "../../components/ui/PlatformDrawer";
import { PlatformDrawerHeader } from "../../components/ui/PlatformDrawerHeader";
import { PlatformModal } from "../../components/ui/PlatformModal";
import { RefreshIcon } from "../../components/ui/RefreshIcon";
import { withRefreshFeedback } from "../../components/ui/refreshFeedback";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { useFeedback } from "../../app/feedback-context";
import { formatApiErrorMessage } from "../../api/client";
import { spaceApi } from "../../api/spaces";
import type { MemberRole, SpaceManagementAPI, SpaceMember, SpaceMemberAPI, SpaceRow } from "../../api/spaces";
import { userApi as defaultUserApi } from "../../api/users";
import type { TenantUserRow, UserManagementAPI } from "../../api/users";

const roleLabels: Record<MemberRole, string> = {
  space_admin: "空间管理员",
  teacher: "教师",
  student: "学生",
};
const userRoleSuggestionLabels: Record<TenantUserRow["role"], string> = {
  tenant_admin: "租户管理员",
  teacher: "教师",
  student: "学生",
};
const statusLabels: Record<SpaceMember["status"], string> = {
  enabled: "启用",
  disabled: "禁用",
};

const listPageSizeOptions = [5, 10, 20, 50];
const defaultListPageSize = 5;
const managementListFetchPageSize = 100;
const addUserSuggestionDebounceMs = 300;

type SpacePageAPI = SpaceManagementAPI & Pick<SpaceMemberAPI, "createSpaceMember" | "listSpaceMembers" | "updateSpaceMember">;

type SpaceManagementPageProps = {
  api?: SpacePageAPI;
  userApi?: Pick<UserManagementAPI, "listUsers">;
  tenantID?: number;
};

export function SpaceManagementPage({ api = spaceApi, userApi = defaultUserApi, tenantID }: SpaceManagementPageProps) {
  const [spaces, setSpaces] = useState<SpaceRow[]>([]);
  const [users, setUsers] = useState<TenantUserRow[]>([]);
  const [spaceName, setSpaceName] = useState("");
  const [spaceDescription, setSpaceDescription] = useState("");
  const [spaceAdminUserID, setSpaceAdminUserID] = useState("");
  const [spaceAdminQuery, setSpaceAdminQuery] = useState("");
  const [debouncedSpaceAdminQuery, setDebouncedSpaceAdminQuery] = useState("");
  const [selectedSpaceAdminName, setSelectedSpaceAdminName] = useState("");
  const [isSpaceAdminInputFocused, setIsSpaceAdminInputFocused] = useState(false);
  const [spaceAdminCandidates, setSpaceAdminCandidates] = useState<TenantUserRow[]>([]);
  const [spaceLogoFileName, setSpaceLogoFileName] = useState("");
  const [logoResetKey, setLogoResetKey] = useState(0);
  const [editingSpace, setEditingSpace] = useState<SpaceRow | null>(null);
  const [editingName, setEditingName] = useState("");
  const [editingDescription, setEditingDescription] = useState("");
  const [editingLogoFileName, setEditingLogoFileName] = useState("");
  const [editingAdminUserID, setEditingAdminUserID] = useState("");
  const [editingAdminQuery, setEditingAdminQuery] = useState("");
  const [debouncedEditingAdminQuery, setDebouncedEditingAdminQuery] = useState("");
  const [selectedEditingAdminName, setSelectedEditingAdminName] = useState("");
  const [isEditingAdminInputFocused, setIsEditingAdminInputFocused] = useState(false);
  const [editingAdminCandidates, setEditingAdminCandidates] = useState<TenantUserRow[]>([]);
  const [loadError, setLoadError] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [spacePage, setSpacePage] = useState(1);
  const [spacePageSize, setSpacePageSize] = useState(defaultListPageSize);
  const [spaceTotal, setSpaceTotal] = useState(0);
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isEditingSpaceDrawerOpen, setIsEditingSpaceDrawerOpen] = useState(false);
  const [isAddUserDialogOpen, setIsAddUserDialogOpen] = useState(false);
  const [addUserQuery, setAddUserQuery] = useState("");
  const [debouncedAddUserQuery, setDebouncedAddUserQuery] = useState("");
  const [selectedAddUserName, setSelectedAddUserName] = useState("");
  const [addUserSpaceID, setAddUserSpaceID] = useState("");
  const [addUserSpaceQuery, setAddUserSpaceQuery] = useState("");
  const [addUserSpaceCandidates, setAddUserSpaceCandidates] = useState<SpaceRow[]>([]);
  const [addUserRole, setAddUserRole] = useState<MemberRole | "">("");
  const [isAddUserInputFocused, setIsAddUserInputFocused] = useState(false);
  const [addUserCandidates, setAddUserCandidates] = useState<TenantUserRow[]>([]);
  const [isAddingUser, setIsAddingUser] = useState(false);
  const [selectedMemberSpaceID, setSelectedMemberSpaceID] = useState<number | null>(null);
  const [isMemberDrawerOpen, setIsMemberDrawerOpen] = useState(false);
  const [memberReloadToken, setMemberReloadToken] = useState(0);
  const [isSpaceListRefreshing, setIsSpaceListRefreshing] = useState(false);
  const [spacePendingDisable, setSpacePendingDisable] = useState<SpaceRow | null>(null);
  const [isDisablingSpace, setIsDisablingSpace] = useState(false);
  const { showError } = useFeedback();

  const selectedMemberSpace = spaces.find((space) => space.id === selectedMemberSpaceID) ?? null;
  const hasTenantContext = isPositiveInteger(tenantID);
  const addUserTargetSpaces = mergeSpaceCandidates(addUserSpaceCandidates, selectedMemberSpace);

  useEffect(() => {
    if (!isPositiveInteger(tenantID)) {
      return;
    }

    let ignore = false;
    const currentTenantID = tenantID;
    Promise.all([
      appliedSearchQuery.trim()
        ? api.listSpaces({ tenantID: currentTenantID, page: spacePage, pageSize: spacePageSize, search: appliedSearchQuery })
        : api.listSpaces({ tenantID: currentTenantID, page: spacePage, pageSize: spacePageSize }),
      userApi.listUsers({ tenantID: currentTenantID, page: 1, pageSize: managementListFetchPageSize }),
    ])
      .then(([spaceData, userData]) => {
        if (!ignore) {
          setSpaces(spaceData.items);
          setSpaceTotal(spaceData.total ?? spaceData.items.length);
          setUsers(userData.items);
          setSpaceAdminUserID((current) => current || String(userData.items[0]?.id ?? ""));
          setLoadError("");
        }
      })
      .catch(() => {
        if (!ignore) {
          setSpaces([]);
          setSpaceTotal(0);
          setLoadError("空间或用户列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, tenantID, userApi, spacePage, spacePageSize, appliedSearchQuery]);

  useEffect(() => {
    if (!isAddUserDialogOpen || addUserQuery.trim() === "") {
      queueMicrotask(() => setDebouncedAddUserQuery(""));
      return;
    }

    const timeoutID = window.setTimeout(() => {
      setDebouncedAddUserQuery(addUserQuery);
    }, addUserSuggestionDebounceMs);

    return () => {
      window.clearTimeout(timeoutID);
    };
  }, [addUserQuery, isAddUserDialogOpen]);

  useEffect(() => {
    if (!isPositiveInteger(tenantID) || !isAddUserDialogOpen) {
      queueMicrotask(() => setAddUserSpaceCandidates([]));
      return;
    }

    let ignore = false;
    api.listSpaces({ tenantID, page: 1, pageSize: 10, search: addUserSpaceQuery, filters: { status: "enabled" } })
      .then((data) => {
        if (!ignore) {
          setAddUserSpaceCandidates(data.items);
        }
      })
      .catch(() => {
        if (!ignore) {
          setAddUserSpaceCandidates([]);
          showError("目标空间候选列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [addUserSpaceQuery, api, isAddUserDialogOpen, showError, tenantID]);

  useEffect(() => {
    if (!isCreateDialogOpen || spaceAdminQuery.trim() === "") {
      queueMicrotask(() => setDebouncedSpaceAdminQuery(""));
      return;
    }

    const timeoutID = window.setTimeout(() => {
      setDebouncedSpaceAdminQuery(spaceAdminQuery);
    }, addUserSuggestionDebounceMs);

    return () => {
      window.clearTimeout(timeoutID);
    };
  }, [isCreateDialogOpen, spaceAdminQuery]);

  useEffect(() => {
    if (!editingSpace || editingAdminQuery.trim() === "") {
      queueMicrotask(() => setDebouncedEditingAdminQuery(""));
      return;
    }

    const timeoutID = window.setTimeout(() => {
      setDebouncedEditingAdminQuery(editingAdminQuery);
    }, addUserSuggestionDebounceMs);

    return () => {
      window.clearTimeout(timeoutID);
    };
  }, [editingAdminQuery, editingSpace]);

  const spacePaginationTotal = spaceTotal;
  const totalSpacePages = Math.max(1, Math.ceil(spacePaginationTotal / spacePageSize));
  const currentSpacePage = Math.min(spacePage, totalSpacePages);
  useEffect(() => {
    if (!isPositiveInteger(tenantID) || !isAddUserDialogOpen || debouncedAddUserQuery.trim() === "") {
      queueMicrotask(() => setAddUserCandidates([]));
      return;
    }

    let ignore = false;
    const targetSpaceID = Number(addUserSpaceID);
    userApi.listUsers({
      tenantID,
      page: 1,
      pageSize: 10,
      search: debouncedAddUserQuery,
      filters: {
        status: "enabled",
        ...(Number.isInteger(targetSpaceID) && targetSpaceID > 0 ? { excludeSpaceID: targetSpaceID } : {}),
      },
    })
      .then((data) => {
        if (!ignore) {
          setAddUserCandidates(data.items);
        }
      })
      .catch(() => {
        if (!ignore) {
          setAddUserCandidates([]);
          showError("用户候选列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [addUserSpaceID, debouncedAddUserQuery, isAddUserDialogOpen, showError, tenantID, userApi]);

  useEffect(() => {
    if (!isPositiveInteger(tenantID) || !isCreateDialogOpen || debouncedSpaceAdminQuery.trim() === "") {
      queueMicrotask(() => setSpaceAdminCandidates([]));
      return;
    }

    let ignore = false;
    userApi.listUsers({
      tenantID,
      page: 1,
      pageSize: 10,
      search: debouncedSpaceAdminQuery,
      filters: { status: "enabled" },
    })
      .then((data) => {
        if (!ignore) {
          setSpaceAdminCandidates(data.items);
        }
      })
      .catch(() => {
        if (!ignore) {
          setSpaceAdminCandidates([]);
          showError("空间管理员候选列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [debouncedSpaceAdminQuery, isCreateDialogOpen, showError, tenantID, userApi]);

  useEffect(() => {
    if (!isPositiveInteger(tenantID) || editingSpace === null || debouncedEditingAdminQuery.trim() === "") {
      queueMicrotask(() => setEditingAdminCandidates([]));
      return;
    }

    let ignore = false;
    userApi.listUsers({
      tenantID,
      page: 1,
      pageSize: 10,
      search: debouncedEditingAdminQuery,
      filters: { status: "enabled" },
    })
      .then((data) => {
        if (!ignore) {
          setEditingAdminCandidates(data.items);
        }
      })
      .catch(() => {
        if (!ignore) {
          setEditingAdminCandidates([]);
          showError("空间管理员候选列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [debouncedEditingAdminQuery, editingSpace, showError, tenantID, userApi]);

  const addUserSuggestions = matchingAddUserSuggestions(addUserCandidates, debouncedAddUserQuery);
  const shouldShowAddUserSuggestions =
    isAddUserInputFocused &&
    addUserQuery.trim() === debouncedAddUserQuery.trim() &&
    debouncedAddUserQuery.trim() !== "" &&
    addUserSuggestions.length > 0;
  const spaceAdminSuggestions = matchingUserSuggestions(spaceAdminCandidates, debouncedSpaceAdminQuery);
  const shouldShowSpaceAdminSuggestions =
    isSpaceAdminInputFocused &&
    spaceAdminQuery.trim() === debouncedSpaceAdminQuery.trim() &&
    debouncedSpaceAdminQuery.trim() !== "" &&
    spaceAdminSuggestions.length > 0;
  const editingAdminSuggestions = matchingUserSuggestions(editingAdminCandidates, debouncedEditingAdminQuery);
  const shouldShowEditingAdminSuggestions =
    isEditingAdminInputFocused &&
    editingAdminQuery.trim() === debouncedEditingAdminQuery.trim() &&
    debouncedEditingAdminQuery.trim() !== "" &&
    editingAdminSuggestions.length > 0;

  async function handleCreateSpace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!isPositiveInteger(tenantID)) {
      setLoadError("当前账号没有租户上下文，请先登录租户账号后再创建空间");
      return;
    }

    const adminUserID = Number(spaceAdminUserID);
    if (!adminUserID) {
      setLoadError("请先选择空间管理员");
      return;
    }

    // 创建空间时必须指定空间管理员，保证空间成员管理从一开始满足管理员不变式。
    await api.createSpace({
      tenantID,
      name: spaceName,
      description: spaceDescription,
      logoFileName: spaceLogoFileName || "未上传",
      adminUserID,
    });

    const spaceData = appliedSearchQuery.trim()
      ? await api.listSpaces({ tenantID, page: 1, pageSize: spacePageSize, search: appliedSearchQuery })
      : await api.listSpaces({ tenantID, page: 1, pageSize: spacePageSize });
    setSpaces(spaceData.items);
    setSpaceTotal(spaceData.total ?? spaceData.items.length);
    setSpacePage(1);
    setIsCreateDialogOpen(false);
  }

  function resetCreateSpaceForm() {
    setSpaceName("");
    setSpaceDescription("");
    setSpaceAdminUserID(String(users[0]?.id ?? ""));
    setSpaceLogoFileName("");
    setLogoResetKey((value) => value + 1);
  }

  function openCreateSpaceDialog() {
    const selectedUser = users.find((user) => String(user.id) === spaceAdminUserID) ?? users[0];
    setSpaceAdminUserID(selectedUser ? String(selectedUser.id) : "");
    setSpaceAdminQuery(selectedUser?.username ?? "");
    setDebouncedSpaceAdminQuery("");
    setSelectedSpaceAdminName(selectedUser?.name ?? "");
    setIsSpaceAdminInputFocused(false);
    setIsCreateDialogOpen(true);
  }

  function openAddUserDialog(spaceID?: number) {
    setAddUserQuery("");
    setDebouncedAddUserQuery("");
    setSelectedAddUserName("");
    setAddUserCandidates([]);
    setAddUserSpaceID(spaceID === undefined ? "" : String(spaceID));
    setAddUserSpaceQuery("");
    setAddUserSpaceCandidates([]);
    setAddUserRole("");
    setIsAddUserInputFocused(false);
    setIsAddUserDialogOpen(true);
  }

  async function handleAddUserToSpace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!isPositiveInteger(tenantID)) {
      setLoadError("当前账号没有租户上下文，请先登录租户账号后再添加用户");
      return;
    }

    const targetSpaceID = Number(addUserSpaceID);
    const keyword = addUserQuery.trim().toLowerCase();
    let targetUser = addUserCandidates.find((user) =>
      user.username.toLowerCase() === keyword || user.name.toLowerCase() === keyword,
    );
    if (!targetUser && keyword !== "" && Number.isInteger(targetSpaceID) && targetSpaceID > 0) {
      const candidateData = await userApi.listUsers({
        tenantID,
        page: 1,
        pageSize: 1,
        search: addUserQuery,
        filters: {
          status: "enabled",
          excludeSpaceID: targetSpaceID,
        },
      });
      targetUser = candidateData.items.find((user) =>
        user.username.toLowerCase() === keyword || user.name.toLowerCase() === keyword,
      );
    }
    if (!targetUser) {
      showError("未找到匹配的租户用户");
      return;
    }

    const targetSpace = addUserTargetSpaces.find((space) => space.id === targetSpaceID);
    if (!targetSpace) {
      showError("请选择目标空间");
      return;
    }
    if (isSpaceDisabled(targetSpace)) {
      showError("空间已禁用，不能继续添加成员");
      return;
    }
    if (!addUserRole) {
      showError("请选择空间身份");
      return;
    }

    setIsAddingUser(true);
    try {
      const nextMember = await api.createSpaceMember({
        tenantID,
        spaceID: targetSpaceID,
        userID: targetUser.id,
        role: addUserRole,
      });

      setSpaces((items) =>
        items.map((space) =>
          space.id === targetSpaceID ? { ...space, members: [...space.members, nextMember] } : space,
        ),
      );
      if (selectedMemberSpaceID === targetSpaceID) {
        setMemberReloadToken((current) => current + 1);
      }
      setIsAddUserDialogOpen(false);
      setAddUserQuery("");
      setDebouncedAddUserQuery("");
      setSelectedAddUserName("");
      setAddUserCandidates([]);
      setAddUserSpaceQuery("");
      setAddUserSpaceCandidates([]);
    } catch (error) {
      showError(formatApiErrorMessage(error, "添加用户到空间失败"));
    } finally {
      setIsAddingUser(false);
    }
  }


  function openSpaceEditor(space: SpaceRow) {
    if (isSpaceDisabled(space)) {
      return;
    }
    const currentAdmin = space.members.find((member) => member.role === "space_admin" && member.status === "enabled");
    setEditingSpace(space);
    setEditingName(space.name);
    setEditingDescription(space.description);
    setEditingLogoFileName(space.logoFileName === "未上传" ? "" : space.logoFileName);
    setEditingAdminUserID(currentAdmin ? String(currentAdmin.userID) : "");
    setEditingAdminQuery(currentAdmin?.username ?? users.find((user) => user.id === currentAdmin?.userID)?.username ?? "");
    setDebouncedEditingAdminQuery("");
    setSelectedEditingAdminName(currentAdmin?.name ?? "");
    setIsEditingAdminInputFocused(false);
    setIsEditingSpaceDrawerOpen(true);
  }

  async function saveSpaceProfile() {
    if (!editingSpace || !isPositiveInteger(tenantID)) {
      return;
    }

    const nextAdminUserID = Number(editingAdminUserID);
    if (!nextAdminUserID) {
      showError("请先选择空间管理员");
      return;
    }

    try {
      const updatedSpace = await api.updateSpace({
        tenantID,
        spaceID: editingSpace.id,
        name: editingName,
        description: editingDescription,
        logoFileName: editingLogoFileName || "未上传",
      });
      const previousAdmin = editingSpace.members.find(
        (member) => member.role === "space_admin" && member.status === "enabled",
      );
      const nextAdmin = editingSpace.members.find((member) => member.userID === nextAdminUserID);
      let nextMembers = mergeSpaceMembers(editingSpace.members, updatedSpace.members);

      if (nextAdminUserID !== previousAdmin?.userID) {
        if (nextAdmin) {
          const promotedMember = await api.updateSpaceMember({
            tenantID,
            spaceID: editingSpace.id,
            userID: nextAdminUserID,
            role: "space_admin",
          });
          nextMembers = upsertSpaceMember(nextMembers, { ...nextAdmin, ...promotedMember });
        } else {
          const createdMember = await api.createSpaceMember({
            tenantID,
            spaceID: editingSpace.id,
            userID: nextAdminUserID,
            role: "space_admin",
          });
          nextMembers = upsertSpaceMember(nextMembers, createdMember);
        }

        if (previousAdmin) {
          const downgradedMember = await api.updateSpaceMember({
            tenantID,
            spaceID: editingSpace.id,
            userID: previousAdmin.userID,
            role: "teacher",
          });
          nextMembers = upsertSpaceMember(nextMembers, { ...previousAdmin, ...downgradedMember, role: "teacher" });
        }
      }

      setSpaces((items) =>
        items.map((space) =>
          space.id === editingSpace.id ? { ...updatedSpace, members: nextMembers } : space,
        ),
      );
      closeSpaceEditor();
    } catch (error) {
      showError(formatApiErrorMessage(error, "保存空间信息失败"));
    }
  }

  function closeSpaceEditor() {
    setIsEditingSpaceDrawerOpen(false);
  }

  function clearSpaceEditorTarget() {
    setEditingSpace(null);
    setEditingName("");
    setEditingDescription("");
    setEditingLogoFileName("");
    setEditingAdminUserID("");
    setEditingAdminQuery("");
    setDebouncedEditingAdminQuery("");
    setSelectedEditingAdminName("");
    setIsEditingAdminInputFocused(false);
  }

  function openMemberManager(spaceID: number) {
    const targetSpace = spaces.find((space) => space.id === spaceID);
    if (!targetSpace || isSpaceDisabled(targetSpace)) {
      return;
    }
    setSelectedMemberSpaceID(spaceID);
    setIsMemberDrawerOpen(true);
  }

  function closeMemberManager() {
    setIsMemberDrawerOpen(false);
  }

  async function confirmDisableSpace() {
    if (!spacePendingDisable || !isPositiveInteger(tenantID)) {
      return;
    }

    setIsDisablingSpace(true);
    try {
      const disabledSpace = await api.disableSpace({ tenantID, spaceID: spacePendingDisable.id });
      setSpaces((items) =>
        items.map((space) =>
          space.id === disabledSpace.id
            ? { ...space, ...disabledSpace, members: mergeSpaceMembers(space.members, disabledSpace.members) }
            : space,
        ),
      );
      if (selectedMemberSpaceID === disabledSpace.id) {
        closeMemberManager();
      }
      if (editingSpace?.id === disabledSpace.id) {
        closeSpaceEditor();
      }
      if (addUserSpaceID === String(disabledSpace.id)) {
        setAddUserSpaceID("");
      }
      setSpacePendingDisable(null);
    } catch (error) {
      showError(formatApiErrorMessage(error, "禁用空间失败"));
    } finally {
      setIsDisablingSpace(false);
    }
  }

  function handleSearchSpaces() {
    setSpacePage(1);
    setAppliedSearchQuery(searchQuery);
  }

  async function handleRefreshSpaces() {
    if (!isPositiveInteger(tenantID)) {
      return;
    }

    setIsSpaceListRefreshing(true);
    try {
      const [spaceData, userData] = await withRefreshFeedback(
        Promise.all([
          api.listSpaces({ tenantID, page: 1, pageSize: spacePageSize }),
          userApi.listUsers({ tenantID, page: 1, pageSize: managementListFetchPageSize }),
        ]),
      );
      setSpaces(spaceData.items);
      setSpaceTotal(spaceData.total ?? spaceData.items.length);
      setUsers(userData.items);
      setSpaceAdminUserID((current) => current || String(userData.items[0]?.id ?? ""));
      setSearchQuery("");
      setAppliedSearchQuery("");
      setSpacePage(1);
      setLoadError("");
    } catch {
      setLoadError("空间或用户列表加载失败");
    } finally {
      setIsSpaceListRefreshing(false);
    }
  }

  if (!hasTenantContext) {
    return (
      <section className="page platform-page tenant-admin-page">
        <nav aria-label="空间管理菜单" className="platform-tabbar" role="tablist">
          <a className="platform-tab platform-tab--active" href="/spaces" role="tab" aria-selected="true">
            空间管理
          </a>
        </nav>

        <Panel>
          <div className="tenant-admin-warning" role="alert">
            当前账号没有租户上下文，请先登录租户账号后再进入空间管理。
          </div>
        </Panel>
      </section>
    );
  }

  return (
    <section className="page platform-page tenant-admin-page">
      <nav aria-label="空间管理菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/spaces" role="tab" aria-selected="true">
          空间管理
        </a>
      </nav>

        <Panel>
          <div className="tenant-list-toolbar">
            <div className="tenant-list-actions" aria-label="空间操作区">
              <Button
                variant="toolbarPrimary"
                onClick={openCreateSpaceDialog}
                type="button"
              >
                创建空间
              </Button>
              <Button
                variant="toolbarSecondary"
                onClick={() => openAddUserDialog()}
                type="button"
              >
                添加用户
              </Button>
            </div>
            <div className="tenant-search-actions">
              <label className="tenant-search-field">
                <span className="sr-only">搜索空间</span>
                <input
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="输入空间名称、描述或管理员"
                  value={searchQuery}
                />
              </label>
              <Button aria-label="搜索" variant="icon" onClick={handleSearchSpaces} type="button">
                <Search aria-hidden="true" size={16} />
              </Button>
              <Button
                aria-label="刷新空间列表"
                disabled={isSpaceListRefreshing}
                variant="icon"
                onClick={() => void handleRefreshSpaces()}
                type="button"
              >
                <RefreshIcon active={isSpaceListRefreshing} />
              </Button>
            </div>
          </div>

          {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
          <div
            className={[
              "table-wrap",
              "tenant-list-transition",
              isSpaceListRefreshing ? "tenant-list-transition--refreshing" : "",
            ].filter(Boolean).join(" ")}
            data-testid="space-list-table-wrap"
          >
            <table className="data-table tenant-admin-table">
              <thead>
                <tr>
                  <th scope="col">空间</th>
                  <th scope="col">Logo</th>
                  <th scope="col">描述</th>
                  <th scope="col">管理员</th>
                  <th scope="col">操作</th>
                </tr>
              </thead>
              <tbody>
                {spaces.length === 0 && <EmptyTableRow colSpan={5} />}
                {spaces.map((space) => {
                  const disabled = isSpaceDisabled(space);
                  return (
                  <tr key={space.id}>
                    <td>
                      <div className="tenant-space-name-cell">
                        <span>{space.name}</span>
                        {disabled && <StatusBadge tone="warning">已禁用</StatusBadge>}
                      </div>
                    </td>
                    <td>{space.logoFileName}</td>
                    <td>{space.description}</td>
                    <td aria-hidden="true">{adminSummary(space)}</td>
                    <td>
                      <div className="tenant-actions">
                        <Button
                          disabled={disabled}
                          variant="actionEdit"
                          onClick={() => openSpaceEditor(space)}
                          type="button"
                        >
                          编辑空间
                        </Button>
                        <Button
                          disabled={disabled}
                          variant="actionReset"
                          onClick={() => openMemberManager(space.id)}
                          type="button"
                        >
                          成员管理
                        </Button>
                        <Button
                          disabled={disabled}
                          variant="actionClose"
                          onClick={() => setSpacePendingDisable(space)}
                          type="button"
                        >
                          禁用空间
                        </Button>
                      </div>
                    </td>
                  </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <Pagination
            onPageSizeChange={(nextPageSize) => {
              setSpacePage(1);
              setSpacePageSize(nextPageSize);
            }}
            page={currentSpacePage}
            pageSize={spacePageSize}
            pageSizeOptions={listPageSizeOptions}
            total={spacePaginationTotal}
            onPageChange={setSpacePage}
          />
        </Panel>

      {selectedMemberSpace && (
        <SpaceMemberDrawer
          afterOpenChange={(nextOpen) => {
            if (!nextOpen) {
              setSelectedMemberSpaceID(null);
            }
          }}
          onClose={closeMemberManager}
          onAddMember={() => openAddUserDialog(selectedMemberSpace.id)}
          api={api}
          open={isMemberDrawerOpen}
          reloadToken={memberReloadToken}
          space={selectedMemberSpace}
          tenantID={tenantID}
        />
      )}

        <PlatformDrawer
          afterOpenChange={(nextOpen) => {
            if (!nextOpen) {
              resetCreateSpaceForm();
            }
          }}
          ariaLabel="创建空间抽屉"
          onClose={() => setIsCreateDialogOpen(false)}
          open={isCreateDialogOpen}
        >
          <PlatformDrawerHeader onBack={() => setIsCreateDialogOpen(false)} title="填写空间资料" />
          <form className="platform-form space-profile-drawer-form" onSubmit={handleCreateSpace}>
            <label className="field">
              <span className="field-label">
                空间名称
                <span className="required-marker" aria-hidden="true">*</span>
              </span>
              <input
                aria-label="空间名称"
                onChange={(event) => setSpaceName(event.target.value)}
                required
                value={spaceName}
              />
            </label>
            <SpaceMarkdownEditor
              label="空间描述"
              onChange={setSpaceDescription}
              required
              value={spaceDescription}
            />
            <label className="field">
              <span className="field-label">
                空间管理员
                <span className="required-marker" aria-hidden="true">*</span>
              </span>
              <AutoComplete
                aria-label="空间管理员"
                className={[
                  "space-admin-autocomplete",
                  selectedSpaceAdminName ? "space-admin-autocomplete--selected" : "",
                ].filter(Boolean).join(" ")}
                getPopupContainer={(trigger) => trigger.parentElement ?? document.body}
                onBlur={() => setIsSpaceAdminInputFocused(false)}
                onChange={(value) => {
                  setSpaceAdminQuery(value);
                  setSpaceAdminUserID("");
                  setSelectedSpaceAdminName("");
                  setSpaceAdminCandidates([]);
                  if (value.trim() === "") {
                    setDebouncedSpaceAdminQuery("");
                  }
                }}
                onFocus={() => setIsSpaceAdminInputFocused(true)}
                onSelect={(value) => {
                  const selectedUser = spaceAdminCandidates.find((user) => user.username === value)
                    ?? users.find((user) => user.username === value);
                  setSpaceAdminUserID(selectedUser ? String(selectedUser.id) : "");
                  setSpaceAdminQuery(value);
                  setSelectedSpaceAdminName(selectedUser?.name ?? "");
                  setIsSpaceAdminInputFocused(false);
                }}
                open={shouldShowSpaceAdminSuggestions}
                options={(spaceAdminQuery.trim() === debouncedSpaceAdminQuery.trim() ? spaceAdminSuggestions : []).map((user) => ({
                  label: renderUserSuggestionOption(user),
                  value: user.username,
                }))}
                placeholder="输入管理员账号或姓名"
                popupRender={(menu) => (
                  <div aria-label="空间管理员候选" role="listbox">
                    {menu}
                  </div>
                )}
                value={spaceAdminQuery}
                virtual={false}
              />
              {selectedSpaceAdminName && (
                <small className="add-user-identity-name" aria-label="已选空间管理员真实姓名">
                  {selectedSpaceAdminName}
                </small>
              )}
            </label>
            <FileUploadField
              accept={["image/png", "image/jpeg"]}
              key={logoResetKey}
              label="空间 Logo"
              maxSizeBytes={1024 * 1024}
              onFileAccepted={(file) => setSpaceLogoFileName(file.name)}
            />
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={() => setIsCreateDialogOpen(false)} type="button">
                取消
              </Button>
              <Button variant="primary" type="submit">
                确认创建
              </Button>
            </div>
          </form>
        </PlatformDrawer>

      <PlatformModal open={isAddUserDialogOpen} onClose={() => setIsAddUserDialogOpen(false)} title="添加用户到空间弹窗">
            <form className="platform-form" onSubmit={(event) => void handleAddUserToSpace(event)}>
              <label className="field">
                <span className="field-label">
                  用户账号或姓名
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <div
                  className={[
                    "add-user-identity-field",
                    selectedAddUserName ? "add-user-identity-field--selected" : "",
                  ].filter(Boolean).join(" ")}
                >
                  <input
                    aria-label="用户账号或姓名"
                    aria-autocomplete="list"
                    aria-controls="add-user-suggestions"
                    aria-expanded={shouldShowAddUserSuggestions}
                    className="add-user-identity-input"
                    onBlur={() => setIsAddUserInputFocused(false)}
                    onChange={(event) => {
                      const value = event.target.value;
                      setAddUserQuery(value);
                      setSelectedAddUserName("");
                      setAddUserCandidates([]);
                      if (value.trim() === "") {
                        setDebouncedAddUserQuery("");
                      }
                    }}
                    onFocus={() => setIsAddUserInputFocused(true)}
                    placeholder="输入已存在用户的账号或姓名"
                    required
                    style={selectedAddUserName ? { width: `${Math.max(addUserQuery.length + 1, 8)}ch` } : undefined}
                    value={addUserQuery}
                  />
                  {selectedAddUserName && (
                    <small className="add-user-identity-name" aria-label="已选用户真实姓名">
                      {selectedAddUserName}
                    </small>
                  )}
                </div>
              </label>
              {shouldShowAddUserSuggestions && (
                <div
                  aria-label="用户账号或姓名候选"
                  className="user-suggestion-list"
                  id="add-user-suggestions"
                  role="listbox"
                >
                  {addUserSuggestions.map((user) => (
                    <button
                      className="user-suggestion-option"
                      key={user.id}
                      onClick={() => {
                        setAddUserQuery(user.username);
                        setSelectedAddUserName(user.name);
                        setIsAddUserInputFocused(false);
                      }}
                      onMouseDown={(event) => event.preventDefault()}
                      role="option"
                      type="button"
                    >
                      <span>{user.name}</span>
                      <small>
                        {user.username} · {userRoleSuggestionLabels[user.role]}
                      </small>
                    </button>
                  ))}
                </div>
              )}
              <label className="field">
                <span className="field-label">搜索目标空间</span>
                <input
                  aria-label="搜索目标空间"
                  onChange={(event) => setAddUserSpaceQuery(event.target.value)}
                  placeholder="输入空间名称"
                  value={addUserSpaceQuery}
                />
              </label>
              <label className="field">
                <span className="field-label">
                  目标空间
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <select
                  aria-label="目标空间"
                  onChange={(event) => setAddUserSpaceID(event.target.value)}
                  required
                  value={addUserSpaceID}
                >
                  <option value="" disabled>
                    请选择空间
                  </option>
                  {addUserTargetSpaces.map((space) => (
                    <option key={space.id} value={space.id}>
                      {space.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field-label">
                  空间身份
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <select
                  aria-label="空间身份"
                  onChange={(event) => setAddUserRole(event.target.value as MemberRole | "")}
                  required
                  value={addUserRole}
                >
                  <option value="" disabled>
                    请选择身份
                  </option>
                  <option value="space_admin">空间管理员</option>
                  <option value="teacher">教师</option>
                  <option value="student">学生</option>
                </select>
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsAddUserDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button disabled={isAddingUser} variant="primary" type="submit">
                  确认添加
                </Button>
              </div>
            </form>
      </PlatformModal>

      <PlatformModal
        open={spacePendingDisable !== null}
        onClose={() => setSpacePendingDisable(null)}
        title="禁用空间风险确认"
      >
        <div className="platform-dialog__warning">
          <p>禁用后该空间下的题库、试卷、考试、阅卷和成绩等业务都将停止操作。</p>
          {spacePendingDisable && <p>即将禁用：{spacePendingDisable.name}</p>}
        </div>
        <div className="platform-dialog__actions">
          <Button disabled={isDisablingSpace} variant="secondary" onClick={() => setSpacePendingDisable(null)} type="button">
            取消
          </Button>
          <Button disabled={isDisablingSpace} variant="primary" onClick={() => void confirmDisableSpace()} type="button">
            确认禁用
          </Button>
        </div>
      </PlatformModal>

      {editingSpace && (
        <PlatformDrawer
          afterOpenChange={(nextOpen) => {
            if (!nextOpen) {
              clearSpaceEditorTarget();
            }
          }}
          ariaLabel="编辑空间抽屉"
          onClose={closeSpaceEditor}
          open={isEditingSpaceDrawerOpen}
        >
          <PlatformDrawerHeader onBack={closeSpaceEditor} title={editingSpace.name} />
          <form className="platform-form space-profile-drawer-form" onSubmit={(event) => {
            event.preventDefault();
            void saveSpaceProfile();
          }}>
            <label className="field">
              <span className="field-label">
                空间名称
                <span className="required-marker" aria-hidden="true">*</span>
              </span>
              <input
                aria-label="编辑空间名称"
                onChange={(event) => setEditingName(event.target.value)}
                required
                value={editingName}
              />
            </label>
            <SpaceMarkdownEditor
              label="编辑空间描述"
              onChange={setEditingDescription}
              required
              value={editingDescription}
            />
            <label className="field">
              <span className="field-label">
                空间管理员
                <span className="required-marker" aria-hidden="true">*</span>
              </span>
              <AutoComplete
                aria-label="编辑空间管理员"
                className={[
                  "space-admin-autocomplete",
                  selectedEditingAdminName ? "space-admin-autocomplete--selected" : "",
                ].filter(Boolean).join(" ")}
                getPopupContainer={(trigger) => trigger.parentElement ?? document.body}
                onBlur={() => setIsEditingAdminInputFocused(false)}
                onChange={(value) => {
                  setEditingAdminQuery(value);
                  setEditingAdminUserID("");
                  setSelectedEditingAdminName("");
                  setEditingAdminCandidates([]);
                  if (value.trim() === "") {
                    setDebouncedEditingAdminQuery("");
                  }
                }}
                onFocus={() => setIsEditingAdminInputFocused(true)}
                onSelect={(value) => {
                  const selectedUser = editingAdminCandidates.find((user) => user.username === value)
                    ?? users.find((user) => user.username === value);
                  setEditingAdminUserID(selectedUser ? String(selectedUser.id) : "");
                  setEditingAdminQuery(value);
                  setSelectedEditingAdminName(selectedUser?.name ?? "");
                  setIsEditingAdminInputFocused(false);
                }}
                open={shouldShowEditingAdminSuggestions}
                options={(editingAdminQuery.trim() === debouncedEditingAdminQuery.trim() ? editingAdminSuggestions : []).map((user) => ({
                  label: renderUserSuggestionOption(user),
                  value: user.username,
                }))}
                placeholder="输入管理员账号或姓名"
                popupRender={(menu) => (
                  <div aria-label="编辑空间管理员候选" role="listbox">
                    {menu}
                  </div>
                )}
                value={editingAdminQuery}
                virtual={false}
              />
              {selectedEditingAdminName && (
                <small className="add-user-identity-name" aria-label="已选编辑空间管理员真实姓名">
                  {selectedEditingAdminName}
                </small>
              )}
            </label>
            <FileUploadField
              accept={["image/png", "image/jpeg"]}
              label="编辑空间 Logo"
              maxSizeBytes={1024 * 1024}
              onFileAccepted={(file) => setEditingLogoFileName(file.name)}
              previewSrc={editingLogoFileName ? resolveSpaceLogoSrc(editingLogoFileName) : undefined}
              selectedLabel={editingLogoFileName ? editingLogoFileName : undefined}
            />
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={closeSpaceEditor} type="button">
                取消
              </Button>
              <Button variant="primary" type="submit">
                保存空间信息
              </Button>
            </div>
          </form>
        </PlatformDrawer>
      )}
    </section>
  );
}

function SpaceMarkdownEditor({
  label,
  onChange,
  required = false,
  value,
}: {
  label: string;
  onChange(value: string): void;
  required?: boolean;
  value: string;
}) {
  return (
    <div className="field markdown-editor space-editor-markdown" data-color-mode="light">
      <span className="field-label">
        空间描述
        {required && <span className="required-marker" aria-hidden="true">*</span>}
      </span>
      <MDEditor
        commandsFilter={localizeMarkdownEditorCommand}
        height="auto"
        onChange={(nextValue) => onChange(nextValue ?? "")}
        preview="live"
        previewOptions={{
          // 空间描述按 Markdown 原文存储，预览阶段复用题干编辑器的公式渲染能力。
          rehypePlugins: [rehypeKatex],
          remarkPlugins: [remarkMath],
        }}
        textareaProps={{
          "aria-label": label,
          required,
        }}
        value={value}
        visibleDragbar={false}
      />
    </div>
  );
}

function renderUserSuggestionOption(user: TenantUserRow) {
  return (
    <div className="user-suggestion-option">
      <span>{user.name}</span>
      <small>
        {user.username} · {userRoleSuggestionLabels[user.role]}
      </small>
    </div>
  );
}

function resolveSpaceLogoSrc(logoURL: string) {
  if (!logoURL.startsWith("/uploads")) {
    return logoURL;
  }

  const apiBaseURL = import.meta.env.VITE_API_BASE_URL ?? "";
  if (!apiBaseURL) {
    return logoURL;
  }

  return `${apiBaseURL.replace(/\/$/, "")}/${logoURL.replace(/^\//, "")}`;
}

type SpaceMemberDrawerProps = {
  afterOpenChange: (open: boolean) => void;
  api: Pick<SpaceMemberAPI, "listSpaceMembers">;
  onAddMember: () => void;
  onClose: () => void;
  open: boolean;
  reloadToken: number;
  space: SpaceRow;
  tenantID: number;
};

function SpaceMemberDrawer({
  afterOpenChange,
  api,
  onAddMember,
  onClose,
  open,
  reloadToken,
  space,
  tenantID,
}: SpaceMemberDrawerProps) {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [members, setMembers] = useState<SpaceMember[]>([]);
  const [memberTotal, setMemberTotal] = useState(0);
  const [memberSearchQuery, setMemberSearchQuery] = useState("");
  const [appliedMemberSearchQuery, setAppliedMemberSearchQuery] = useState("");
  const [memberPage, setMemberPage] = useState(1);
  const [memberPageSize, setMemberPageSize] = useState(defaultListPageSize);
  const [isMemberListRefreshing, setIsMemberListRefreshing] = useState(false);
  const closeDrawer = () => {
    onClose();
  };
  const totalMemberPages = Math.max(1, Math.ceil(memberTotal / memberPageSize));
  const currentMemberPage = Math.min(memberPage, totalMemberPages);

  useEffect(() => {
    if (!open) {
      return;
    }

    let ignore = false;
    queueMicrotask(() => {
      if (!ignore) {
        setIsMemberListRefreshing(true);
      }
    });
    api.listSpaceMembers({
      tenantID,
      spaceID: space.id,
      page: memberPage,
      pageSize: memberPageSize,
      search: appliedMemberSearchQuery,
    })
      .then((data) => {
        if (!ignore) {
          setMembers(data.items);
          setMemberTotal(data.total ?? data.items.length);
        }
      })
      .catch(() => {
        if (!ignore) {
          setMembers([]);
          setMemberTotal(0);
        }
      })
      .finally(() => {
        if (!ignore) {
          setIsMemberListRefreshing(false);
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, tenantID, space.id, open, memberPage, memberPageSize, appliedMemberSearchQuery, reloadToken]);

  const searchMembers = () => {
    setAppliedMemberSearchQuery(memberSearchQuery);
    setMemberPage(1);
  };

  const refreshMembers = async () => {
    setIsMemberListRefreshing(true);
    try {
      setMemberSearchQuery("");
      setAppliedMemberSearchQuery("");
      setMemberPage(1);
      const data = await withRefreshFeedback(api.listSpaceMembers({
        tenantID,
        spaceID: space.id,
        page: 1,
        pageSize: memberPageSize,
        search: "",
      }));
      setMembers(data.items);
      setMemberTotal(data.total ?? data.items.length);
    } finally {
      setIsMemberListRefreshing(false);
    }
  };

  return (
    <PlatformDrawer
      afterOpenChange={afterOpenChange}
      ariaLabel={`${space.name}成员抽屉`}
      fullscreen={isFullscreen}
      onClose={closeDrawer}
      open={open}
    >
        <div data-testid="space-member-drawer-header">
          <PlatformDrawerHeader
            backAriaLabel="返回空间列表"
            onBack={closeDrawer}
            title={space.name}
            actions={(
              <>
            <button
              aria-label={isFullscreen ? "退出全屏抽屉" : "全屏抽屉"}
              className="tenant-resource-drawer__icon"
              onClick={() => setIsFullscreen((current) => !current)}
              type="button"
            >
              {isFullscreen
                ? <Minimize2 aria-hidden="true" size={18} />
                : <Maximize2 aria-hidden="true" size={18} />}
            </button>
              </>
            )}
          />
        </div>

        <div className="tenant-resource-drawer__body" data-testid="space-member-drawer-body">
          <div
            className="tenant-resource-drawer__toolbar tenant-resource-drawer__toolbar--member-search"
            data-testid="space-member-toolbar"
          >
            <Button onClick={onAddMember} type="button" variant="toolbarPrimary">
              <Plus aria-hidden="true" size={15} />
              添加成员
            </Button>
            <div className="tenant-search-actions">
              <label className="tenant-search-field">
                <span className="sr-only">成员检索关键词</span>
                <input
                  onChange={(event) => setMemberSearchQuery(event.target.value)}
                  placeholder="输入成员姓名、角色或状态"
                  value={memberSearchQuery}
                />
              </label>
              <Button aria-label="搜索成员" variant="icon" onClick={searchMembers} type="button">
                <Search aria-hidden="true" size={16} />
              </Button>
              <Button
                aria-label="刷新成员列表"
                disabled={isMemberListRefreshing}
                variant="icon"
                onClick={() => void refreshMembers()}
                type="button"
              >
                <RefreshIcon active={isMemberListRefreshing} />
              </Button>
            </div>
          </div>
          <div
            className={[
              "table-wrap",
              "tenant-resource-drawer__table",
              "tenant-list-transition",
              isMemberListRefreshing ? "tenant-list-transition--refreshing" : "",
            ].filter(Boolean).join(" ")}
            data-testid="space-member-table-wrap"
          >
            <table className="data-table tenant-resource-table">
              <thead>
                <tr>
                  <th scope="col">成员</th>
                  <th scope="col">角色</th>
                  <th scope="col">状态</th>
                </tr>
              </thead>
              <tbody>
                {members.length === 0 && <EmptyTableRow colSpan={3} />}
                {members.map((member) => (
                  <tr key={member.id}>
                    <td>{member.name}</td>
                    <td>{roleLabels[member.role]}</td>
                    <td>
                      <StatusBadge tone={member.status === "enabled" ? "success" : "warning"}>
                        {statusLabels[member.status]}
                      </StatusBadge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination
            onPageSizeChange={(nextPageSize) => {
              setMemberPage(1);
              setMemberPageSize(nextPageSize);
            }}
            page={currentMemberPage}
            pageSize={memberPageSize}
            pageSizeOptions={listPageSizeOptions}
            total={memberTotal}
            onPageChange={setMemberPage}
          />
        </div>
      </PlatformDrawer>
  );
}


function isPositiveInteger(value: number | undefined): value is number {
  return Number.isInteger(value) && Number(value) > 0;
}

function isSpaceDisabled(space: SpaceRow) {
  return space.status === "disabled";
}

function adminSummary(space: SpaceRow) {
  const admins = space.members
    .filter((member) => member.role === "space_admin" && member.status === "enabled")
    .map((member) => member.name)
    .join("、");

  return `空间管理员：${admins}`;
}

function mergeSpaceMembers(currentMembers: SpaceMember[], incomingMembers: SpaceMember[]) {
  if (incomingMembers.length === 0) {
    return currentMembers;
  }

  return incomingMembers.map((member) => currentMembers.find((item) => item.userID === member.userID) ?? member);
}

function upsertSpaceMember(members: SpaceMember[], nextMember: SpaceMember) {
  const exists = members.some((member) => member.userID === nextMember.userID);
  if (!exists) {
    return [...members, nextMember];
  }

  return members.map((member) => (member.userID === nextMember.userID ? nextMember : member));
}

function matchingAddUserSuggestions(
  users: TenantUserRow[],
  query: string,
) {
  const keyword = query.trim().toLowerCase();

  return users
    .filter((user) =>
      user.status === "enabled" &&
      (keyword === "" || user.username.toLowerCase().includes(keyword) || user.name.toLowerCase().includes(keyword)),
    );
}

function mergeSpaceCandidates(candidates: SpaceRow[], selectedSpace: SpaceRow | null) {
  if (selectedSpace === null || candidates.some((space) => space.id === selectedSpace.id)) {
    return candidates;
  }
  return [selectedSpace, ...candidates];
}

function matchingUserSuggestions(users: TenantUserRow[], query: string) {
  const keyword = query.trim().toLowerCase();
  if (keyword === "") {
    return [];
  }

  return users
    .filter((user) =>
      user.status === "enabled" &&
      (user.username.toLowerCase().includes(keyword) || user.name.toLowerCase().includes(keyword)),
    );
}
