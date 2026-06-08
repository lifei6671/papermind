package exam

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	"github.com/lifei6671/papermind/server/library/constant"
)

func TestCreateDraftAndPublishExamValidatesSettingsAndFreezesRuleLivePool(t *testing.T) {
	repo := &fakeRepository{
		inviteCodes: map[string]bool{"DUPLICATE": true},
		papers: map[uint64]Paper{
			100: {ID: 100, BuildMode: BuildModeRuleLive, Status: constant.PaperStatusEnabled},
			101: {ID: 101, BuildMode: BuildModeManual, ContainsShortText: true, Status: constant.PaperStatusEnabled},
		},
		liveCandidates: []LivePoolItem{
			{SectionID: 10, RuleID: 1, QuestionID: 300},
		},
	}
	generator := &fakeCodeGenerator{codes: []string{"DUPLICATE", "INVITE001"}}
	svc := NewService(ServiceOptions{
		Repo:          repo,
		CodeGenerator: generator,
		TokenIssuer:   fakeTokenIssuer{token: "exam-token"},
		Now:           fixedNow,
	})

	draft, err := svc.CreateDraft(context.Background(), CreateDraftInput{
		TenantID: 10,
		PaperID:  100,
		Name:     "期中考试",
	})
	if err != nil {
		t.Fatalf("CreateDraft returned error: %v", err)
	}
	if draft.Status != StatusDraft || draft.MaxAttempts != 1 || draft.ResultStrategy != ResultStrategyLatest {
		t.Fatalf("expected draft defaults, got %#v", draft)
	}

	_, err = svc.Publish(context.Background(), PublishInput{
		TenantID:        10,
		ExamID:          1,
		PaperID:         100,
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 30*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     1,
		ResultStrategy:  ResultStrategyLatest,
		PublishMode:     PublishModeImmediateScore,
	})
	if !errors.Is(err, ErrDurationExceedsExamWindow) {
		t.Fatalf("expected ErrDurationExceedsExamWindow, got %v", err)
	}

	_, err = svc.Publish(context.Background(), PublishInput{
		TenantID:        10,
		ExamID:          2,
		PaperID:         101,
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 120*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     2,
		ResultStrategy:  ResultStrategyHighest,
		PublishMode:     PublishModeManualPublish,
	})
	if !errors.Is(err, ErrShortTextCannotRepeatAttempt) {
		t.Fatalf("expected ErrShortTextCannotRepeatAttempt, got %v", err)
	}

	_, err = svc.Publish(context.Background(), PublishInput{
		TenantID:        10,
		ExamID:          3,
		PaperID:         101,
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 120*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     1,
		ResultStrategy:  ResultStrategyLatest,
		PublishMode:     PublishModeImmediateScore,
	})
	if !errors.Is(err, ErrShortTextCannotImmediateScore) {
		t.Fatalf("expected ErrShortTextCannotImmediateScore, got %v", err)
	}

	scorePublishTime := fixedUnixMilli + 90*minuteMillis
	published, err := svc.Publish(context.Background(), PublishInput{
		TenantID:         10,
		ExamID:           1,
		PaperID:          100,
		StartTime:        fixedUnixMilli,
		EndTime:          fixedUnixMilli + 120*minuteMillis,
		DurationMinutes:  60,
		MaxAttempts:      1,
		ResultStrategy:   ResultStrategyLatest,
		PublishMode:      PublishModeManualPublish,
		ScorePublishTime: &scorePublishTime,
	})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if published.InviteCode != "INVITE001" {
		t.Fatalf("expected unique invite code INVITE001, got %q", published.InviteCode)
	}
	if published.Status != StatusPublished || published.ScorePublishTime == nil || *published.ScorePublishTime != scorePublishTime {
		t.Fatalf("expected published exam with score publish time, got %#v", published)
	}
	if !repo.frozeLivePool || len(repo.frozenPool) != 1 {
		t.Fatalf("expected rule_live pool frozen, got %#v", repo.frozenPool)
	}
}

func TestPublishRejectsPaperThatIsNotEnabled(t *testing.T) {
	repo := &fakeRepository{
		papers: map[uint64]Paper{
			100: {ID: 100, BuildMode: BuildModeManual, Status: constant.PaperStatusDisabled},
			101: {ID: 101, BuildMode: BuildModeManual, Status: constant.PaperStatusDraft},
		},
	}
	svc := NewService(ServiceOptions{
		Repo:          repo,
		CodeGenerator: &fakeCodeGenerator{codes: []string{"INVITE001"}},
		TokenIssuer:   fakeTokenIssuer{token: "exam-token"},
		Now:           fixedNow,
	})

	for _, paperID := range []uint64{100, 101} {
		_, err := svc.Publish(context.Background(), PublishInput{
			TenantID:        10,
			ExamID:          1,
			PaperID:         paperID,
			StartTime:       fixedUnixMilli,
			EndTime:         fixedUnixMilli + 120*minuteMillis,
			DurationMinutes: 60,
			MaxAttempts:     1,
			ResultStrategy:  ResultStrategyLatest,
			PublishMode:     PublishModeManualPublish,
		})
		if !errors.Is(err, ErrPaperNotEnabled) {
			t.Fatalf("paper %d: expected ErrPaperNotEnabled, got %v", paperID, err)
		}
	}
	if repo.updatedExam.ID != 0 {
		t.Fatalf("expected non-enabled paper to stop publish before persistence, got %#v", repo.updatedExam)
	}
}

func TestTargetsRejectDuplicatesAndInviteRequiresLogin(t *testing.T) {
	repo := &fakeRepository{
		targets: map[targetKey]bool{
			{tenantID: 10, examID: 1, targetType: TargetTypeSpace, targetID: 100}: true,
		},
		examsByInvite: map[string]Exam{
			"INVITE001": {ID: 1, TenantID: 10, Status: StatusPublished},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo, Now: fixedNow})

	err := svc.AddTarget(context.Background(), AddTargetInput{
		TenantID:   10,
		ExamID:     1,
		TargetType: TargetTypeSpace,
		TargetID:   100,
	})
	if !errors.Is(err, ErrDuplicateExamTarget) {
		t.Fatalf("expected ErrDuplicateExamTarget, got %v", err)
	}

	if err := svc.AddTarget(context.Background(), AddTargetInput{TenantID: 10, ExamID: 1, TargetType: TargetTypeUser, TargetID: 20}); err != nil {
		t.Fatalf("AddTarget user returned error: %v", err)
	}
	if repo.addedTarget.TargetType != TargetTypeUser || repo.addedTarget.TargetID != 20 {
		t.Fatalf("expected user target added, got %#v", repo.addedTarget)
	}

	_, err = svc.ResolveInvite(context.Background(), ResolveInviteInput{InviteCode: "INVITE001"})
	if !errors.Is(err, ErrLoginRequiredForInvite) {
		t.Fatalf("expected ErrLoginRequiredForInvite, got %v", err)
	}

	exam, err := svc.ResolveInvite(context.Background(), ResolveInviteInput{InviteCode: "INVITE001", UserID: 20})
	if err != nil {
		t.Fatalf("ResolveInvite returned error: %v", err)
	}
	if exam.ID != 1 {
		t.Fatalf("expected exam ID 1, got %d", exam.ID)
	}
}

func TestPublishWithTargetSupportsMultipleTargetsAndKeepsSingleTargetCompatibility(t *testing.T) {
	repo := &fakeRepository{
		papers: map[uint64]Paper{
			100: {ID: 100, BuildMode: BuildModeManual, Status: constant.PaperStatusEnabled},
		},
	}
	svc := NewService(ServiceOptions{
		Repo:          repo,
		CodeGenerator: &fakeCodeGenerator{codes: []string{"INVITE001", "INVITE002"}},
		Now:           fixedNow,
	})

	_, err := svc.PublishWithTarget(context.Background(), PublishWithTargetInput{
		TenantID:        10,
		PaperID:         100,
		Name:            "多目标考试",
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 120*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     1,
		ResultStrategy:  ResultStrategyLatest,
		PublishMode:     PublishModeManualPublish,
		Targets: []Target{
			{TargetType: TargetTypeSpace, TargetID: 301},
			{TargetType: TargetTypeSpace, TargetID: 301},
			{TargetType: TargetTypeUser, TargetID: 21, ScopeSpaceIDs: []uint64{301}},
		},
	})
	if err != nil {
		t.Fatalf("PublishWithTarget multiple targets returned error: %v", err)
	}
	if len(repo.addedTargets) != 2 {
		t.Fatalf("expected duplicate target to be removed, got %#v", repo.addedTargets)
	}
	if repo.addedTargets[0].TenantID != 10 || repo.addedTargets[0].TargetType != TargetTypeSpace || repo.addedTargets[0].TargetID != 301 {
		t.Fatalf("unexpected first target: %#v", repo.addedTargets[0])
	}
	if repo.addedTargets[1].TenantID != 10 || repo.addedTargets[1].TargetType != TargetTypeUser || repo.addedTargets[1].TargetID != 21 {
		t.Fatalf("unexpected second target: %#v", repo.addedTargets[1])
	}
	if len(repo.addedTargets[1].ScopeSpaceIDs) != 1 || repo.addedTargets[1].ScopeSpaceIDs[0] != 301 {
		t.Fatalf("expected scoped user target preserved, got %#v", repo.addedTargets[1])
	}

	_, err = svc.PublishWithTarget(context.Background(), PublishWithTargetInput{
		TenantID:        10,
		PaperID:         100,
		Name:            "兼容单目标考试",
		TargetType:      TargetTypeSpace,
		TargetID:        302,
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 120*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     1,
		ResultStrategy:  ResultStrategyLatest,
		PublishMode:     PublishModeManualPublish,
	})
	if err != nil {
		t.Fatalf("PublishWithTarget single target returned error: %v", err)
	}
	if len(repo.addedTargets) != 1 || repo.addedTargets[0].TargetID != 302 {
		t.Fatalf("expected compatible single target, got %#v", repo.addedTargets)
	}
}

func TestPublishWithTargetRejectsEmptyTargets(t *testing.T) {
	repo := &fakeRepository{
		papers: map[uint64]Paper{
			100: {ID: 100, BuildMode: BuildModeManual, Status: constant.PaperStatusEnabled},
		},
	}
	svc := NewService(ServiceOptions{
		Repo:          repo,
		CodeGenerator: &fakeCodeGenerator{codes: []string{"INVITE001"}},
		Now:           fixedNow,
	})

	_, err := svc.PublishWithTarget(context.Background(), PublishWithTargetInput{
		TenantID:        10,
		PaperID:         100,
		Name:            "缺少目标考试",
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 120*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     1,
		ResultStrategy:  ResultStrategyLatest,
		PublishMode:     PublishModeManualPublish,
	})
	if !errors.Is(err, ErrExamTargetRequired) {
		t.Fatalf("expected ErrExamTargetRequired, got %v", err)
	}
	if repo.updatedExam.ID != 0 {
		t.Fatalf("expected empty target to stop before persistence, got %#v", repo.updatedExam)
	}
}

func TestManagementDetailAllowsTenantAdminFullScope(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		managementTargets: map[uint64][]Target{
			501: {
				{TenantID: 10, ExamID: 501, TargetType: TargetTypeSpace, TargetID: 301},
				{TenantID: 10, ExamID: 501, TargetType: TargetTypeUser, TargetID: 21},
			},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301, 302},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
		Now:               fixedNow,
	})

	detail, err := svc.GetDetail(context.Background(), ManagementDetailInput{
		TenantID: 10,
		ExamID:   501,
		Permission: permission.PermissionContext{
			SubjectType: permission.SubjectTenantUser,
			TenantID:    10,
			UserID:      11,
			Role:        permission.RoleTenantAdmin,
		},
	})
	if err != nil {
		t.Fatalf("GetDetail returned error: %v", err)
	}
	if detail.Exam.ID != 501 || len(detail.Targets) != 2 {
		t.Fatalf("expected exam and original targets, got %#v", detail)
	}
	if got := detail.AllowedSpaceIDs; len(got) != 2 || got[0] != 301 || got[1] != 302 {
		t.Fatalf("expected tenant admin to keep all target spaces, got %#v", got)
	}
	if !detail.Permissions.CanViewDetail || !detail.Permissions.CanExportResults || !detail.Permissions.CanUpdateSettings {
		t.Fatalf("expected tenant admin management permissions, got %#v", detail.Permissions)
	}
}

func TestManagementDetailIntersectsSpaceRoleWithExamTargetSpaces(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		managementTargets: map[uint64][]Target{
			501: {{TenantID: 10, ExamID: 501, TargetType: TargetTypeSpace, TargetID: 301}},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301, 302},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})

	detail, err := svc.GetDetail(context.Background(), ManagementDetailInput{
		TenantID: 10,
		ExamID:   501,
		Permission: permission.PermissionContext{
			SubjectType: permission.SubjectTenantUser,
			TenantID:    10,
			UserID:      12,
			Role:        permission.RoleTeacher,
			SpaceMemberships: map[uint64]string{
				301: permission.RoleTeacher,
				999: permission.RoleTeacher,
			},
		},
	})
	if err != nil {
		t.Fatalf("GetDetail returned error: %v", err)
	}
	if got := detail.AllowedSpaceIDs; len(got) != 1 || got[0] != 301 {
		t.Fatalf("expected teacher to see only intersected target space, got %#v", got)
	}
	if !detail.Permissions.CanViewResults || detail.Permissions.CanExportResults || detail.Permissions.CanUpdateSettings {
		t.Fatalf("expected teacher read permissions without export/settings, got %#v", detail.Permissions)
	}
}

func TestManagementDetailKeepsSpaceAdminCandidateActionsScopedButBlocksGlobalSettingsWrites(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		managementTargets: map[uint64][]Target{
			501: {{TenantID: 10, ExamID: 501, TargetType: TargetTypeSpace, TargetID: 301}},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})

	detail, err := svc.GetDetail(context.Background(), ManagementDetailInput{
		TenantID: 10,
		ExamID:   501,
		Permission: permission.PermissionContext{
			SubjectType: permission.SubjectTenantUser,
			TenantID:    10,
			UserID:      12,
			Role:        permission.RoleSpaceAdmin,
			SpaceMemberships: map[uint64]string{
				301: permission.RoleSpaceAdmin,
			},
		},
	})
	if err != nil {
		t.Fatalf("GetDetail returned error: %v", err)
	}
	if !detail.Permissions.CanViewResults || !detail.Permissions.CanExportResults {
		t.Fatalf("expected scoped space admin to keep result visibility/export, got %#v", detail.Permissions)
	}
	if !detail.Permissions.CanManageCandidates {
		t.Fatalf("expected scoped space admin to keep candidate management actions, got %#v", detail.Permissions)
	}
	if detail.Permissions.CanPublishResults || detail.Permissions.CanUpdateSettings {
		t.Fatalf("expected scoped space admin to be blocked from global result/settings writes, got %#v", detail.Permissions)
	}
}

func TestManagementLogsAndOverviewDeduplicateMultiSpaceOperationGroups(t *testing.T) {
	space301 := uint64(301)
	space302 := uint64(302)
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		managementTargets: map[uint64][]Target{
			501: {{TenantID: 10, ExamID: 501, TargetType: TargetTypeSpace, TargetID: 301}},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301, 302},
		},
		operationLogs: []OperationLog{
			{
				ID:              9001,
				TenantID:        10,
				ExamID:          501,
				OperationType:   OperationTypeSendInvite,
				OperationTitle:  "重发邀请码",
				OperationDetail: "重发 2 名考生邀请码",
				ActorRole:       permission.RoleSpaceAdmin,
				SpaceID:         &space301,
				CreatedAt:       1_700_000_000_000,
				ExtJSON:         `{"operation_group_id":"invite-group-1"}`,
			},
			{
				ID:              9002,
				TenantID:        10,
				ExamID:          501,
				OperationType:   OperationTypeSendInvite,
				OperationTitle:  "重发邀请码",
				OperationDetail: "重发 2 名考生邀请码",
				ActorRole:       permission.RoleSpaceAdmin,
				SpaceID:         &space302,
				CreatedAt:       1_700_000_000_000,
				ExtJSON:         `{"operation_group_id":"invite-group-1"}`,
			},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})
	permissionContext := permission.PermissionContext{
		SubjectType: permission.SubjectTenantUser,
		TenantID:    10,
		UserID:      12,
		Role:        permission.RoleSpaceAdmin,
		SpaceMemberships: map[uint64]string{
			301: permission.RoleSpaceAdmin,
			302: permission.RoleSpaceAdmin,
		},
	}

	logs, err := svc.ListLogs(context.Background(), OperationLogListInput{
		ManagementDetailInput: ManagementDetailInput{
			TenantID:   10,
			ExamID:     501,
			Permission: permissionContext,
		},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListLogs returned error: %v", err)
	}
	if logs.Total != 1 || len(logs.Items) != 1 {
		t.Fatalf("expected grouped multi-space logs to dedupe to one row, got total=%d items=%#v", logs.Total, logs.Items)
	}

	overview, err := svc.GetOverview(context.Background(), ManagementDetailInput{
		TenantID:   10,
		ExamID:     501,
		Permission: permissionContext,
	})
	if err != nil {
		t.Fatalf("GetOverview returned error: %v", err)
	}
	if len(overview.RecentActivities) != 1 {
		t.Fatalf("expected grouped recent activities to dedupe to one row, got %#v", overview.RecentActivities)
	}
}

func TestManagementLogsKeepFullTotalAfterCrossSpaceMergePagination(t *testing.T) {
	space301 := uint64(301)
	space302 := uint64(302)
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		managementTargets: map[uint64][]Target{
			501: {{TenantID: 10, ExamID: 501, TargetType: TargetTypeSpace, TargetID: 301}},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301, 302},
		},
		operationLogs: []OperationLog{
			{ID: 9001, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_500, ExtJSON: `{"operation_group_id":"group-5"}`},
			{ID: 9002, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_500, ExtJSON: `{"operation_group_id":"group-5"}`},
			{ID: 9003, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_400, ExtJSON: `{"operation_group_id":"group-4"}`},
			{ID: 9004, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_400, ExtJSON: `{"operation_group_id":"group-4"}`},
			{ID: 9005, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_300, ExtJSON: `{"operation_group_id":"group-3"}`},
			{ID: 9006, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_300, ExtJSON: `{"operation_group_id":"group-3"}`},
			{ID: 9007, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_200, ExtJSON: `{"operation_group_id":"group-2"}`},
			{ID: 9008, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_200, ExtJSON: `{"operation_group_id":"group-2"}`},
			{ID: 9009, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_100, ExtJSON: `{"operation_group_id":"group-1"}`},
			{ID: 9010, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_100, ExtJSON: `{"operation_group_id":"group-1"}`},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})
	permissionContext := permission.PermissionContext{
		SubjectType: permission.SubjectTenantUser,
		TenantID:    10,
		UserID:      12,
		Role:        permission.RoleSpaceAdmin,
		SpaceMemberships: map[uint64]string{
			301: permission.RoleSpaceAdmin,
			302: permission.RoleSpaceAdmin,
		},
	}

	logs, err := svc.ListLogs(context.Background(), OperationLogListInput{
		ManagementDetailInput: ManagementDetailInput{
			TenantID:   10,
			ExamID:     501,
			Permission: permissionContext,
		},
		Page:     1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListLogs returned error: %v", err)
	}
	if logs.Total != 5 {
		t.Fatalf("expected full deduped total 5, got %d", logs.Total)
	}
	if len(logs.Items) != 2 || logs.Items[0].ID != 9002 || logs.Items[1].ID != 9004 {
		t.Fatalf("expected first page to keep newest grouped logs, got %#v", logs.Items)
	}
}

func TestManagementLogsDeduplicateBeforePageTruncation(t *testing.T) {
	space301 := uint64(301)
	space302 := uint64(302)
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		managementTargets: map[uint64][]Target{
			501: {{TenantID: 10, ExamID: 501, TargetType: TargetTypeSpace, TargetID: 301}},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301, 302},
		},
		operationLogs: []OperationLog{
			{ID: 9101, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_500, ExtJSON: `{"operation_group_id":"group-5"}`},
			{ID: 9102, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_499, ExtJSON: `{"operation_group_id":"group-5"}`},
			{ID: 9103, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_498, ExtJSON: `{"operation_group_id":"group-5"}`},
			{ID: 9104, TenantID: 10, ExamID: 501, OperationType: OperationTypeImportCandidates, OperationTitle: "导入考生", SpaceID: &space301, CreatedAt: 1_700_000_000_400, ExtJSON: `{"operation_group_id":"group-4"}`},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})
	permissionContext := permission.PermissionContext{
		SubjectType: permission.SubjectTenantUser,
		TenantID:    10,
		UserID:      12,
		Role:        permission.RoleSpaceAdmin,
		SpaceMemberships: map[uint64]string{
			301: permission.RoleSpaceAdmin,
			302: permission.RoleSpaceAdmin,
		},
	}

	logs, err := svc.ListLogs(context.Background(), OperationLogListInput{
		ManagementDetailInput: ManagementDetailInput{
			TenantID:   10,
			ExamID:     501,
			Permission: permissionContext,
		},
		Page:     1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListLogs returned error: %v", err)
	}
	if logs.Total != 2 {
		t.Fatalf("expected grouped total 2, got %d", logs.Total)
	}
	if len(logs.Items) != 2 || logs.Items[0].ID != 9101 || logs.Items[1].ID != 9104 {
		t.Fatalf("expected first page to keep two newest unique groups, got %#v", logs.Items)
	}
}

func TestManagementLogsTenantAdminDeduplicatesOperationGroups(t *testing.T) {
	space301 := uint64(301)
	space302 := uint64(302)
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		managementTargets: map[uint64][]Target{
			501: {{TenantID: 10, ExamID: 501, TargetType: TargetTypeSpace, TargetID: 301}},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301, 302},
		},
		operationLogs: []OperationLog{
			{ID: 9001, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_500, ExtJSON: `{"operation_group_id":"group-5"}`},
			{ID: 9002, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_500, ExtJSON: `{"operation_group_id":"group-5"}`},
			{ID: 9003, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_400, ExtJSON: `{"operation_group_id":"group-4"}`},
			{ID: 9004, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_400, ExtJSON: `{"operation_group_id":"group-4"}`},
			{ID: 9005, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_300, ExtJSON: `{"operation_group_id":"group-3"}`},
			{ID: 9006, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_300, ExtJSON: `{"operation_group_id":"group-3"}`},
			{ID: 9007, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_200, ExtJSON: `{"operation_group_id":"group-2"}`},
			{ID: 9008, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_200, ExtJSON: `{"operation_group_id":"group-2"}`},
			{ID: 9009, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space301, CreatedAt: 1_700_000_000_100, ExtJSON: `{"operation_group_id":"group-1"}`},
			{ID: 9010, TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "重发邀请码", SpaceID: &space302, CreatedAt: 1_700_000_000_100, ExtJSON: `{"operation_group_id":"group-1"}`},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})

	logs, err := svc.ListLogs(context.Background(), OperationLogListInput{
		ManagementDetailInput: ManagementDetailInput{
			TenantID: 10,
			ExamID:   501,
			Permission: permission.PermissionContext{
				SubjectType: permission.SubjectTenantUser,
				TenantID:    10,
				UserID:      11,
				Role:        permission.RoleTenantAdmin,
			},
		},
		Page:     1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListLogs returned error: %v", err)
	}
	if logs.Total != 5 {
		t.Fatalf("expected grouped total 5 for tenant admin, got %d", logs.Total)
	}
	if len(logs.Items) != 2 || logs.Items[0].ID != 9002 || logs.Items[1].ID != 9004 {
		t.Fatalf("expected tenant admin first page to dedupe newest groups, got %#v", logs.Items)
	}

	overview, err := svc.GetOverview(context.Background(), ManagementDetailInput{
		TenantID: 10,
		ExamID:   501,
		Permission: permission.PermissionContext{
			SubjectType: permission.SubjectTenantUser,
			TenantID:    10,
			UserID:      11,
			Role:        permission.RoleTenantAdmin,
		},
	})
	if err != nil {
		t.Fatalf("GetOverview returned error: %v", err)
	}
	if len(overview.RecentActivities) != 4 {
		t.Fatalf("expected tenant admin overview to keep 4 grouped activities, got %#v", overview.RecentActivities)
	}
	if overview.RecentActivities[0].CreatedAt != 1_700_000_000_500 ||
		overview.RecentActivities[1].CreatedAt != 1_700_000_000_400 ||
		overview.RecentActivities[2].CreatedAt != 1_700_000_000_300 ||
		overview.RecentActivities[3].CreatedAt != 1_700_000_000_200 {
		t.Fatalf("expected tenant admin overview to dedupe by group and keep newest groups, got %#v", overview.RecentActivities)
	}
}

func TestManagementDetailRejectsStudentAndOutOfScopeActor(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})

	_, err := svc.GetDetail(context.Background(), ManagementDetailInput{
		TenantID: 10,
		ExamID:   501,
		Permission: permission.PermissionContext{
			SubjectType: permission.SubjectTenantUser,
			TenantID:    10,
			UserID:      21,
			Role:        permission.RoleStudent,
			SpaceMemberships: map[uint64]string{
				301: permission.RoleStudent,
			},
		},
	})
	if !errors.Is(err, permission.ErrForbidden) {
		t.Fatalf("expected student to be forbidden, got %v", err)
	}

	_, err = svc.GetDetail(context.Background(), ManagementDetailInput{
		TenantID: 10,
		ExamID:   501,
		Permission: permission.PermissionContext{
			SubjectType: permission.SubjectTenantUser,
			TenantID:    10,
			UserID:      12,
			Role:        permission.RoleTeacher,
			SpaceMemberships: map[uint64]string{
				999: permission.RoleTeacher,
			},
		},
	})
	if !errors.Is(err, permission.ErrForbidden) {
		t.Fatalf("expected out-of-scope teacher to be forbidden, got %v", err)
	}
}

func TestManagementDetailAllowsAuthorizedEmptyCollections(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})

	detail, err := svc.GetDetail(context.Background(), ManagementDetailInput{
		TenantID: 10,
		ExamID:   501,
		Permission: permission.PermissionContext{
			SubjectType: permission.SubjectTenantUser,
			TenantID:    10,
			UserID:      12,
			Role:        permission.RoleSpaceAdmin,
			SpaceMemberships: map[uint64]string{
				301: permission.RoleSpaceAdmin,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected authorized empty target list to be visible, got %v", err)
	}
	if len(detail.Targets) != 0 || len(detail.AllowedSpaceIDs) != 1 || detail.AllowedSpaceIDs[0] != 301 {
		t.Fatalf("expected empty targets with authorized space scope, got %#v", detail)
	}
}

func TestManagementOverviewActivitiesRespectAuthorizedSpaces(t *testing.T) {
	spaceID := uint64(301)
	otherSpaceID := uint64(302)
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "高一数学月考", Status: StatusPublished},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301},
		},
		candidateCount: 32,
		attemptStats:   AttemptOverviewStats{Joined: 12, Submitted: 8, InProgress: 4},
		operationLogs: []OperationLog{
			{TenantID: 10, ExamID: 501, OperationType: OperationTypePublishExam, OperationTitle: "发布到授权空间", SpaceID: &spaceID, CreatedAt: 2000},
			{TenantID: 10, ExamID: 501, OperationType: OperationTypeSendInvite, OperationTitle: "其它空间邀请", SpaceID: &otherSpaceID, CreatedAt: 1900},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})

	overview, err := svc.GetOverview(context.Background(), ManagementDetailInput{
		TenantID: 10,
		ExamID:   501,
		Permission: permission.PermissionContext{
			SubjectType: permission.SubjectTenantUser,
			TenantID:    10,
			UserID:      12,
			Role:        permission.RoleSpaceAdmin,
			SpaceMemberships: map[uint64]string{
				301: permission.RoleSpaceAdmin,
			},
		},
	})
	if err != nil {
		t.Fatalf("GetOverview returned error: %v", err)
	}
	if len(overview.RecentActivities) != 1 || overview.RecentActivities[0].OperationTitle != "发布到授权空间" {
		t.Fatalf("expected only authorized space activity, got %#v", overview.RecentActivities)
	}
	if len(repo.listOperationLogInputs) != 1 || repo.listOperationLogInputs[0].SpaceID == nil || *repo.listOperationLogInputs[0].SpaceID != 301 {
		t.Fatalf("expected operation logs to be queried with authorized space, got %#v", repo.listOperationLogInputs)
	}
}

func TestManagementPaperPreviewUsesFrozenPoolForRuleLive(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "动态考试", Status: StatusPublished, BuildMode: BuildModeRuleLive},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301},
		},
		liveQuestions: []SnapshotSourceQuestion{
			{SectionID: 10, SectionName: "一、单选题", QuestionID: 300, QuestionType: QuestionTypeSingle, Title: "冻结题干", Score: "2", CorrectText: "不应返回"},
		},
		fixedQuestions: []SnapshotSourceQuestion{
			{SectionID: 10, SectionName: "一、单选题", QuestionID: 999, QuestionType: QuestionTypeSingle, Title: "实时题干", Score: "2"},
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
	})

	preview, err := svc.GetPaperPreview(context.Background(), PaperPreviewInput{
		ManagementDetailInput: ManagementDetailInput{
			TenantID: 10,
			ExamID:   501,
			Permission: permission.PermissionContext{
				SubjectType: permission.SubjectTenantUser,
				TenantID:    10,
				UserID:      11,
				Role:        permission.RoleTenantAdmin,
			},
		},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("GetPaperPreview returned error: %v", err)
	}
	if !repo.usedFrozenPool || repo.usedFixedQuestions {
		t.Fatalf("expected rule_live preview to use frozen pool only, usedFrozen=%v usedFixed=%v", repo.usedFrozenPool, repo.usedFixedQuestions)
	}
	if preview.Total != 1 || len(preview.Items) != 1 || preview.Items[0].QuestionID != 300 || preview.Items[0].Title != "冻结题干" {
		t.Fatalf("expected frozen preview question, got %#v", preview)
	}
}

func TestManagementImportCandidatesAddsUserTargetsAndRejectsStartedExam(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "未开考考试", Status: StatusPublished, StartTime: fixedUnixMilli + minuteMillis},
			502: {ID: 502, TenantID: 10, PaperID: 100, Name: "已开考考试", Status: StatusPublished, StartTime: fixedUnixMilli - minuteMillis},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301},
			502: {301},
		},
		importExistingTargets: map[targetKey]bool{
			{tenantID: 10, examID: 501, targetType: TargetTypeUser, targetID: 21}: true,
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
		Now:               fixedNow,
	})
	permissionContext := permission.PermissionContext{
		SubjectType: permission.SubjectTenantUser,
		TenantID:    10,
		UserID:      11,
		Role:        permission.RoleTenantAdmin,
	}

	result, err := svc.ImportCandidates(context.Background(), ImportCandidatesInput{
		ManagementDetailInput: ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   10,
			ExamID:     501,
		},
		UserIDs: []uint64{21, 22, 22},
	})
	if err != nil {
		t.Fatalf("ImportCandidates returned error: %v", err)
	}
	if result.ImportedCount != 1 || result.SkippedCount != 2 {
		t.Fatalf("expected one new target and two skipped users, got %#v", result)
	}
	if len(repo.importedCandidateTargets) != 1 || repo.importedCandidateTargets[0].TargetType != TargetTypeUser || repo.importedCandidateTargets[0].TargetID != 22 {
		t.Fatalf("expected only non-duplicate user target 22 to be imported, got %#v", repo.importedCandidateTargets)
	}
	if len(repo.operationLogs) != 1 || repo.operationLogs[0].OperationType != OperationTypeImportCandidates || repo.operationLogs[0].ActorID != 11 {
		t.Fatalf("expected import_candidates operation log, got %#v", repo.operationLogs)
	}

	_, err = svc.ImportCandidates(context.Background(), ImportCandidatesInput{
		ManagementDetailInput: ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   10,
			ExamID:     502,
		},
		UserIDs: []uint64{23},
	})
	if !errors.Is(err, ErrExamAlreadyStarted) {
		t.Fatalf("expected ErrExamAlreadyStarted, got %v", err)
	}
	if len(repo.importInputs) != 1 {
		t.Fatalf("started exam should stop before repository write, import inputs=%#v", repo.importInputs)
	}
}

func TestManagementResendInvitationsFiltersCandidatesAndRejectsStartedExam(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			501: {ID: 501, TenantID: 10, PaperID: 100, Name: "未开考考试", Status: StatusPublished, InviteCode: "PM2026", StartTime: fixedUnixMilli + minuteMillis},
			502: {ID: 502, TenantID: 10, PaperID: 100, Name: "已开考考试", Status: StatusPublished, InviteCode: "PM2027", StartTime: fixedUnixMilli - minuteMillis},
		},
		targetSpaceIDs: map[uint64][]uint64{
			501: {301},
			502: {301},
		},
		resendCandidateIDs: map[uint64]bool{
			21: true,
			22: true,
		},
	}
	svc := NewManagementDetailService(ManagementDetailServiceOptions{
		Repo:              repo,
		PermissionChecker: permission.NewFixedRoleChecker(),
		Now:               fixedNow,
	})
	permissionContext := permission.PermissionContext{
		SubjectType: permission.SubjectTenantUser,
		TenantID:    10,
		UserID:      11,
		Role:        permission.RoleTenantAdmin,
	}

	result, err := svc.ResendInvitations(context.Background(), ResendInvitationsInput{
		ManagementDetailInput: ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   10,
			ExamID:     501,
		},
		UserIDs: []uint64{21, 22, 22, 99},
	})
	if err != nil {
		t.Fatalf("ResendInvitations returned error: %v", err)
	}
	if result.SentCount != 2 || result.SkippedCount != 2 || result.InviteCode != "PM2026" {
		t.Fatalf("expected two sent, two skipped and invite code, got %#v", result)
	}
	if len(repo.resendInputs) != 1 || len(repo.resendInputs[0].UserIDs) != 3 {
		t.Fatalf("expected deduplicated repository input once, got %#v", repo.resendInputs)
	}
	if len(repo.operationLogs) != 1 || repo.operationLogs[0].OperationType != OperationTypeSendInvite || repo.operationLogs[0].ActorID != 11 {
		t.Fatalf("expected send_invite operation log, got %#v", repo.operationLogs)
	}

	_, err = svc.ResendInvitations(context.Background(), ResendInvitationsInput{
		ManagementDetailInput: ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   10,
			ExamID:     502,
		},
		UserIDs: []uint64{21},
	})
	if !errors.Is(err, ErrExamAlreadyStarted) {
		t.Fatalf("expected ErrExamAlreadyStarted, got %v", err)
	}
	if len(repo.resendInputs) != 1 {
		t.Fatalf("started exam should stop before repository resend, resend inputs=%#v", repo.resendInputs)
	}
}

func TestStartExamIsIdempotentAndIssuesOpaqueAttemptToken(t *testing.T) {
	existingTokenHash := HashExamToken("existing-token")
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			1: {ID: 1, TenantID: 10, PaperID: 100, StartTime: fixedUnixMilli - minuteMillis, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30, MaxAttempts: 2, Status: StatusPublished},
		},
		eligible: true,
		existingInProgress: &Attempt{
			ID:                 99,
			TenantID:           10,
			ExamID:             1,
			UserID:             20,
			AttemptNo:          1,
			Status:             AttemptStatusInProgress,
			ExamTokenHash:      existingTokenHash,
			ExamTokenExpiresAt: fixedUnixMilli + 30*minuteMillis + defaultTokenBufferMillis,
		},
	}
	svc := NewService(ServiceOptions{
		Repo:        repo,
		TokenIssuer: fakeTokenIssuer{token: "exam-token"},
		Now:         fixedNow,
	})

	started, err := svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if err != nil {
		t.Fatalf("StartExam returned error: %v", err)
	}
	if started.Attempt.ID != 99 || repo.createdAttempt.ID != 0 {
		t.Fatalf("expected existing in_progress attempt, got started=%#v created=%#v", started.Attempt, repo.createdAttempt)
	}
	if started.ExamToken != "exam-token" {
		t.Fatalf("expected existing attempt to receive a fresh token, got %q", started.ExamToken)
	}
	if started.Attempt.ExamTokenHash != HashExamToken("exam-token") || started.Attempt.ExamTokenHash == existingTokenHash || repo.updateAttemptTokenCount != 1 {
		t.Fatalf("expected existing attempt token to be refreshed, got started=%#v updates=%d", started, repo.updateAttemptTokenCount)
	}

	repo.existingInProgress = nil
	repo.attemptCount = 2
	_, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if !errors.Is(err, ErrMaxAttemptsReached) {
		t.Fatalf("expected ErrMaxAttemptsReached, got %v", err)
	}

	repo.attemptCount = 1
	repo.createdAttempt = Attempt{}
	started, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if err != nil {
		t.Fatalf("StartExam create returned error: %v", err)
	}
	if started.ExamToken != "exam-token" {
		t.Fatalf("expected opaque token returned, got %q", started.ExamToken)
	}
	if repo.createdAttempt.AttemptNo != 2 {
		t.Fatalf("expected next attempt no 2, got %d", repo.createdAttempt.AttemptNo)
	}
	if repo.createdAttempt.ExamTokenHash == "" || repo.createdAttempt.ExamTokenHash == "exam-token" {
		t.Fatalf("expected only token hash saved, got %q", repo.createdAttempt.ExamTokenHash)
	}
	if repo.createdAttempt.ExamTokenExpiresAt != fixedUnixMilli+30*minuteMillis+defaultTokenBufferMillis {
		t.Fatalf("unexpected token expiry: %d", repo.createdAttempt.ExamTokenExpiresAt)
	}

	repo.conflictOnCreate = true
	repo.existingInProgress = nil
	repo.conflictCreated = false
	started, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if err != nil {
		t.Fatalf("StartExam conflict returned error: %v", err)
	}
	if started.Attempt.ID != 100 || repo.retriedAfterConflict != 1 {
		t.Fatalf("expected conflict to query existing in_progress only, got attempt=%#v retries=%d", started.Attempt, repo.retriedAfterConflict)
	}
	if started.ExamToken != "exam-token" || repo.updateAttemptTokenCount != 2 {
		t.Fatalf("expected conflict path to renew token, got token=%q updates=%d", started.ExamToken, repo.updateAttemptTokenCount)
	}
}

func TestStartExamUsesConfiguredTokenBuffer(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			1: {ID: 1, TenantID: 10, PaperID: 100, StartTime: fixedUnixMilli - minuteMillis, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30, MaxAttempts: 1, Status: StatusPublished},
		},
		eligible: true,
	}
	svc := NewService(ServiceOptions{
		Repo:                   repo,
		TokenIssuer:            fakeTokenIssuer{token: "exam-token"},
		Now:                    fixedNow,
		ExamTokenBufferMinutes: 30,
	})

	_, err := svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if err != nil {
		t.Fatalf("StartExam returned error: %v", err)
	}

	wantExpiresAt := fixedUnixMilli + 30*minuteMillis + 30*minuteMillis
	if repo.createdAttempt.ExamTokenExpiresAt != wantExpiresAt {
		t.Fatalf("ExamTokenExpiresAt = %d, want %d", repo.createdAttempt.ExamTokenExpiresAt, wantExpiresAt)
	}
}

func TestStartExamValidatesQualificationAndTime(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			1: {ID: 1, TenantID: 10, StartTime: fixedUnixMilli + minuteMillis, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30, MaxAttempts: 1, Status: StatusPublished},
			2: {ID: 2, TenantID: 10, StartTime: fixedUnixMilli - 60*minuteMillis, EndTime: fixedUnixMilli - minuteMillis, DurationMinutes: 30, MaxAttempts: 1, Status: StatusPublished},
		},
		eligible: false,
	}
	svc := NewService(ServiceOptions{Repo: repo, TokenIssuer: fakeTokenIssuer{token: "exam-token"}, Now: fixedNow})

	_, err := svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if !errors.Is(err, ErrExamNotEligible) {
		t.Fatalf("expected ErrExamNotEligible, got %v", err)
	}

	repo.eligible = true
	_, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if !errors.Is(err, ErrExamNotStarted) {
		t.Fatalf("expected ErrExamNotStarted, got %v", err)
	}

	_, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 2, UserID: 20})
	if !errors.Is(err, ErrExamEnded) {
		t.Fatalf("expected ErrExamEnded, got %v", err)
	}
}

func TestGenerateAttemptSnapshotsUseSourceAndKeepImmutablePayload(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			1: {ID: 1, TenantID: 10, PaperID: 100, BuildMode: BuildModeManual},
			2: {ID: 2, TenantID: 10, PaperID: 101, BuildMode: BuildModeRuleFixed},
			3: {ID: 3, TenantID: 10, PaperID: 102, BuildMode: BuildModeRuleLive},
		},
		fixedQuestions: []SnapshotSourceQuestion{
			{SectionID: 10, SectionName: "一、单选题", QuestionID: 300, QuestionType: QuestionTypeSingle, Title: "1+1=?", Score: "2", OptionIDs: []uint64{2, 1}, CorrectOptionIDs: []uint64{1}},
			{SectionID: 11, SectionName: "二、填空题", QuestionID: 301, QuestionType: QuestionTypeFillBlank, Title: "Go mod file", Score: "3", CorrectText: "go.mod"},
		},
		liveQuestions: []SnapshotSourceQuestion{
			{SectionID: 12, SectionName: "实时题", QuestionID: 400, QuestionType: QuestionTypeSingle, Title: "live", Score: "5", OptionIDs: []uint64{9, 8}, CorrectOptionIDs: []uint64{8}},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo, Now: fixedNow})

	for _, examID := range []uint64{1, 2} {
		snapshots, err := svc.GenerateAttemptSnapshots(context.Background(), GenerateSnapshotInput{TenantID: 10, ExamID: examID, AttemptID: 900})
		if err != nil {
			t.Fatalf("GenerateAttemptSnapshots fixed exam %d returned error: %v", examID, err)
		}
		if !repo.usedFixedQuestions {
			t.Fatalf("expected manual/rule_fixed to use fixed paper section questions")
		}
		if len(snapshots) != 2 || snapshots[0].SortOrder != 1 || snapshots[1].SortOrder != 2 {
			t.Fatalf("expected global continuous sort order, got %#v", snapshots)
		}
		if snapshots[0].SectionSnapshot == "" || snapshots[0].QuestionSnapshot == "" || snapshots[0].OptionSnapshot == "" || snapshots[0].CorrectAnswerSnapshot == "" {
			t.Fatalf("expected all snapshot payloads generated, got %#v", snapshots[0])
		}
		if snapshots[0].OptionSnapshot == "[A,B]" {
			t.Fatalf("expected option snapshot to store option IDs, not frontend labels")
		}
	}

	repo.usedFixedQuestions = false
	snapshots, err := svc.GenerateAttemptSnapshots(context.Background(), GenerateSnapshotInput{TenantID: 10, ExamID: 3, AttemptID: 901})
	if err != nil {
		t.Fatalf("GenerateAttemptSnapshots rule_live returned error: %v", err)
	}
	if !repo.usedFrozenPool || repo.usedFixedQuestions {
		t.Fatalf("expected rule_live to use frozen pool only")
	}
	if len(snapshots) != 1 || snapshots[0].QuestionID != 400 {
		t.Fatalf("expected live snapshot from frozen pool, got %#v", snapshots)
	}
	var questionSnapshot struct {
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal([]byte(snapshots[0].QuestionSnapshot), &questionSnapshot); err != nil {
		t.Fatalf("question snapshot should be valid JSON: %v", err)
	}
	if questionSnapshot.Type != QuestionTypeSingle {
		t.Fatalf("expected question type frozen into snapshot, got %#v", questionSnapshot)
	}
	before := snapshots[0].QuestionSnapshot
	repo.liveQuestions[0].Title = "changed"
	if snapshots[0].QuestionSnapshot != before {
		t.Fatalf("expected generated snapshot to be immutable after question bank change")
	}
}

func TestExamTokenScopeOnlyCurrentAttempt(t *testing.T) {
	repo := &fakeRepository{
		attemptsByTokenHash: map[string]Attempt{},
	}
	svc := NewService(ServiceOptions{Repo: repo, TokenIssuer: fakeTokenIssuer{token: "exam-token"}, Now: fixedNow})
	hash := svc.HashExamToken("exam-token")
	repo.attemptsByTokenHash[hash] = Attempt{ID: 99, TenantID: 10, ExamID: 1, UserID: 20, Status: AttemptStatusInProgress, ExamTokenHash: hash, ExamTokenExpiresAt: fixedUnixMilli + minuteMillis}

	attempt, err := svc.ValidateExamToken(context.Background(), "exam-token", 99)
	if err != nil {
		t.Fatalf("ValidateExamToken returned error: %v", err)
	}
	if attempt.ID != 99 {
		t.Fatalf("expected attempt 99, got %d", attempt.ID)
	}

	_, err = svc.ValidateExamToken(context.Background(), "exam-token", 100)
	if !errors.Is(err, ErrExamTokenAttemptMismatch) {
		t.Fatalf("expected ErrExamTokenAttemptMismatch, got %v", err)
	}
}

const (
	fixedUnixMilli           int64 = 1767225600000
	minuteMillis             int64 = 60 * 1000
	defaultTokenBufferMillis int64 = 5 * minuteMillis
)

func fixedNow() int64 {
	return fixedUnixMilli
}

type fakeRepository struct {
	papers         map[uint64]Paper
	exams          map[uint64]Exam
	inviteCodes    map[string]bool
	liveCandidates []LivePoolItem
	frozeLivePool  bool
	frozenPool     []LivePoolItem
	updatedExam    Exam

	targets                  map[targetKey]bool
	addedTarget              Target
	addedTargets             []Target
	importExistingTargets    map[targetKey]bool
	importInputs             []ImportCandidateTargetsInput
	importedCandidateTargets []Target
	resendCandidateIDs       map[uint64]bool
	resendInputs             []ResendInvitationsRepositoryInput
	examsByInvite            map[string]Exam
	managementTargets        map[uint64][]Target
	targetSpaceIDs           map[uint64][]uint64
	candidateCount           int
	attemptStats             AttemptOverviewStats
	candidates               []ExamCandidate
	resultSummary            ResultSummaryRepositoryResult
	results                  []ExamResult
	answerSheet              AnswerSheetRepositoryResult
	operationLogs            []OperationLog
	listOperationLogInputs   []ListOperationLogsInput

	eligible                bool
	existingInProgress      *Attempt
	attemptCount            int
	createdAttempt          Attempt
	updateAttemptTokenCount int
	conflictOnCreate        bool
	conflictCreated         bool
	retriedAfterConflict    int
	attemptsByTokenHash     map[string]Attempt

	fixedQuestions     []SnapshotSourceQuestion
	liveQuestions      []SnapshotSourceQuestion
	usedFixedQuestions bool
	usedFrozenPool     bool
}

func (r *fakeRepository) ListExams(ctx context.Context, input ListInput) (pagination.Result[Exam], error) {
	exams := make([]Exam, 0, len(r.exams))
	for _, exam := range r.exams {
		if exam.TenantID == input.TenantID {
			exams = append(exams, exam)
		}
	}
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	return pagination.Result[Exam]{
		Items:    exams,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    int64(len(exams)),
	}, nil
}

func (r *fakeRepository) CreateExam(ctx context.Context, exam Exam) (Exam, error) {
	exam.ID = 1
	return exam, nil
}

func (r *fakeRepository) GetPaper(ctx context.Context, tenantID uint64, paperID uint64) (Paper, error) {
	return r.papers[paperID], nil
}

func (r *fakeRepository) InviteCodeExists(ctx context.Context, tenantID uint64, code string) (bool, error) {
	return r.inviteCodes[code], nil
}

func (r *fakeRepository) ListRuleLiveCandidates(ctx context.Context, tenantID uint64, paperID uint64) ([]LivePoolItem, error) {
	return r.liveCandidates, nil
}

func (r *fakeRepository) PublishExamAndFreezeLivePool(ctx context.Context, exam Exam, pool []LivePoolItem) (Exam, error) {
	r.updatedExam = exam
	r.frozeLivePool = len(pool) > 0
	r.frozenPool = append([]LivePoolItem(nil), pool...)
	return exam, nil
}

func (r *fakeRepository) CreatePublishedExamWithTarget(ctx context.Context, exam Exam, pool []LivePoolItem, target Target, log *OperationLog) (Exam, error) {
	return r.CreatePublishedExamWithTargets(ctx, exam, pool, []Target{target}, log)
}

func (r *fakeRepository) CreatePublishedExamWithTargets(ctx context.Context, exam Exam, pool []LivePoolItem, targets []Target, log *OperationLog) (Exam, error) {
	if exam.ID == 0 {
		exam.ID = 1
	}
	r.updatedExam = exam
	r.frozeLivePool = len(pool) > 0
	r.frozenPool = append([]LivePoolItem(nil), pool...)
	r.addedTargets = make([]Target, 0, len(targets))
	for _, target := range targets {
		target.ExamID = exam.ID
		r.addedTargets = append(r.addedTargets, target)
	}
	if len(r.addedTargets) > 0 {
		r.addedTarget = r.addedTargets[0]
	}
	if log != nil {
		publishLog := *log
		publishLog.ExamID = exam.ID
		// fake 仓储同步记录发布日志，保证 service 测试能发现审计日志遗漏。
		r.operationLogs = append(r.operationLogs, publishLog)
	}
	return exam, nil
}

func (r *fakeRepository) TargetExists(ctx context.Context, tenantID uint64, examID uint64, targetType string, targetID uint64) (bool, error) {
	return r.targets[targetKey{tenantID: tenantID, examID: examID, targetType: targetType, targetID: targetID}], nil
}

func (r *fakeRepository) AddTarget(ctx context.Context, target Target) error {
	r.addedTarget = target
	return nil
}

func (r *fakeRepository) FindExamByInviteCode(ctx context.Context, inviteCode string) (Exam, error) {
	return r.examsByInvite[inviteCode], nil
}

func (r *fakeRepository) GetExam(ctx context.Context, tenantID uint64, examID uint64) (Exam, error) {
	return r.exams[examID], nil
}

func (r *fakeRepository) ListTargets(ctx context.Context, tenantID uint64, examID uint64) ([]Target, error) {
	// 测试仓储返回副本，避免 service 内部排序或裁剪时污染用例夹具。
	targets := make([]Target, len(r.managementTargets[examID]))
	copy(targets, r.managementTargets[examID])
	return targets, nil
}

func (r *fakeRepository) ExamTargetSpaceIDs(ctx context.Context, tenantID uint64, examID uint64) ([]uint64, error) {
	// 真实仓储会把空间投放和用户直投统一展开为有效空间，这里只模拟展开后的结果。
	spaceIDs := make([]uint64, len(r.targetSpaceIDs[examID]))
	copy(spaceIDs, r.targetSpaceIDs[examID])
	return spaceIDs, nil
}

func (r *fakeRepository) CountExamCandidates(ctx context.Context, tenantID uint64, examID uint64, spaceIDs []uint64) (int, error) {
	return r.candidateCount, nil
}

func (r *fakeRepository) CountExamAttemptStats(ctx context.Context, tenantID uint64, examID uint64, spaceIDs []uint64) (AttemptOverviewStats, error) {
	return r.attemptStats, nil
}

func (r *fakeRepository) ListExamCandidates(ctx context.Context, input ListExamCandidatesInput) (pagination.Result[ExamCandidate], error) {
	items := make([]ExamCandidate, len(r.candidates))
	copy(items, r.candidates)
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	return pagination.Result[ExamCandidate]{
		Items:    items,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    int64(len(items)),
	}, nil
}

func (r *fakeRepository) SummarizeExamResults(ctx context.Context, input ResultSummaryRepositoryInput) (ResultSummaryRepositoryResult, error) {
	// fake 仓储直接返回测试预置摘要，避免 service 单测依赖真实 SQL 聚合。
	return r.resultSummary, nil
}

func (r *fakeRepository) ListExamResults(ctx context.Context, input ListExamResultsInput) (pagination.Result[ExamResult], error) {
	// fake 仓储只模拟分页外壳，成绩排序和权限过滤由 DAO 测试覆盖。
	items := make([]ExamResult, len(r.results))
	copy(items, r.results)
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	return pagination.Result[ExamResult]{
		Items:    items,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    int64(len(items)),
	}, nil
}

func (r *fakeRepository) GetAnswerSheet(ctx context.Context, input AnswerSheetRepositoryInput) (AnswerSheetRepositoryResult, error) {
	// fake 仓储直接返回预置答卷详情，双重定位和权限过滤由 API/DAO 测试覆盖。
	return r.answerSheet, nil
}

func (r *fakeRepository) UpdateManagementSettings(ctx context.Context, input UpdateManagementSettingsRepositoryInput) (Exam, error) {
	// fake 仓储只模拟发布配置更新和日志追加；事务原子性由 DAO/API 测试覆盖。
	exam := r.exams[input.ExamID]
	exam.PublishMode = input.PublishMode
	exam.ScorePublishTime = input.ScorePublishTime
	r.exams[input.ExamID] = exam
	if input.Log.OperationType != "" {
		r.operationLogs = append(r.operationLogs, input.Log)
	}
	return exam, nil
}

func (r *fakeRepository) ImportCandidateTargets(ctx context.Context, input ImportCandidateTargetsInput) (ImportCandidateTargetsResult, error) {
	r.importInputs = append(r.importInputs, input)
	result := ImportCandidateTargetsResult{}
	for _, userID := range input.UserIDs {
		key := targetKey{tenantID: input.TenantID, examID: input.ExamID, targetType: TargetTypeUser, targetID: userID}
		if r.importExistingTargets[key] {
			result.SkippedCount++
			continue
		}
		target := Target{TenantID: input.TenantID, ExamID: input.ExamID, TargetType: TargetTypeUser, TargetID: userID}
		r.importedCandidateTargets = append(r.importedCandidateTargets, target)
		result.ImportedTargets = append(result.ImportedTargets, target)
		result.ImportedCount++
	}
	if input.Log.OperationType != "" {
		r.operationLogs = append(r.operationLogs, input.Log)
	}
	return result, nil
}

func (r *fakeRepository) ResendInvitations(ctx context.Context, input ResendInvitationsRepositoryInput) (ResendInvitationsRepositoryResult, error) {
	r.resendInputs = append(r.resendInputs, input)
	result := ResendInvitationsRepositoryResult{}
	for _, userID := range input.UserIDs {
		// fake 仓储用显式集合模拟“当前授权范围内真实应考名单”，保证 service 测试不依赖 SQL 细节。
		if !r.resendCandidateIDs[userID] {
			result.SkippedCount++
			continue
		}
		result.SentUserIDs = append(result.SentUserIDs, userID)
		result.SentCount++
	}
	if input.Log.OperationType != "" {
		r.operationLogs = append(r.operationLogs, input.Log)
	}
	return result, nil
}

func (r *fakeRepository) ListOperationLogs(ctx context.Context, input ListOperationLogsInput) (pagination.Result[OperationLog], error) {
	r.listOperationLogInputs = append(r.listOperationLogInputs, input)
	items := make([]OperationLog, 0, len(r.operationLogs))
	for _, log := range r.operationLogs {
		if log.TenantID != input.TenantID || log.ExamID != input.ExamID {
			continue
		}
		if input.OperationType != "" && log.OperationType != input.OperationType {
			continue
		}
		// 真实仓储对 space_id 使用精确过滤，fake 仓储同步该行为以覆盖权限裁剪。
		if input.SpaceID != nil {
			if log.SpaceID == nil || *log.SpaceID != *input.SpaceID {
				continue
			}
		}
		items = append(items, log)
	}
	sortOperationLogsDesc(items)
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	offset := pagination.Offset(page)
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + page.PageSize
	if end > len(items) {
		end = len(items)
	}
	return pagination.Result[OperationLog]{
		Items:    items[offset:end],
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    int64(len(items)),
	}, nil
}

func (r *fakeRepository) IsEligible(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (bool, error) {
	return r.eligible, nil
}

func (r *fakeRepository) FindInProgressAttempt(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (Attempt, error) {
	if r.conflictCreated {
		r.retriedAfterConflict++
	}
	if r.existingInProgress == nil {
		return Attempt{}, ErrAttemptNotFound
	}
	return *r.existingInProgress, nil
}

func (r *fakeRepository) CountAttempts(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (int, error) {
	return r.attemptCount, nil
}

func (r *fakeRepository) CreateAttempt(ctx context.Context, attempt Attempt) (Attempt, error) {
	if r.conflictOnCreate {
		r.conflictCreated = true
		r.existingInProgress = &Attempt{ID: 100, TenantID: attempt.TenantID, ExamID: attempt.ExamID, UserID: attempt.UserID, AttemptNo: attempt.AttemptNo, Status: AttemptStatusInProgress}
		return Attempt{}, ErrAttemptUniqueConflict
	}
	attempt.ID = 101
	r.createdAttempt = attempt
	return attempt, nil
}

func (r *fakeRepository) UpdateAttemptToken(ctx context.Context, attempt Attempt) (Attempt, error) {
	r.updateAttemptTokenCount++
	if r.existingInProgress != nil && r.existingInProgress.ID == attempt.ID {
		*r.existingInProgress = attempt
	}
	return attempt, nil
}

func (r *fakeRepository) FindAttemptByTokenHash(ctx context.Context, tokenHash string) (Attempt, error) {
	attempt, ok := r.attemptsByTokenHash[tokenHash]
	if !ok {
		return Attempt{}, ErrAttemptNotFound
	}
	return attempt, nil
}

func (r *fakeRepository) ListFixedSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error) {
	r.usedFixedQuestions = true
	return append([]SnapshotSourceQuestion(nil), r.fixedQuestions...), nil
}

func (r *fakeRepository) ListFrozenLiveSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error) {
	r.usedFrozenPool = true
	return append([]SnapshotSourceQuestion(nil), r.liveQuestions...), nil
}

func (r *fakeRepository) SaveAttemptQuestions(ctx context.Context, questions []AttemptQuestion) ([]AttemptQuestion, error) {
	return append([]AttemptQuestion(nil), questions...), nil
}

func (r *fakeRepository) UpdateScorePublishConfig(ctx context.Context, tenantID uint64, examID uint64, publishMode string, scorePublishTime *int64, log *OperationLog) (Exam, error) {
	exam := r.exams[examID]
	exam.PublishMode = publishMode
	exam.ScorePublishTime = scorePublishTime
	r.exams[examID] = exam
	if log != nil {
		publishLog := *log
		publishLog.ExamID = examID
		// fake 仓储记录成绩发布日志，避免 service 测试遗漏发布审计。
		r.operationLogs = append(r.operationLogs, publishLog)
	}
	return exam, nil
}

type fakeCodeGenerator struct {
	codes []string
	next  int
}

func (g *fakeCodeGenerator) NextCode() (string, error) {
	code := g.codes[g.next]
	g.next++
	return code, nil
}

type fakeTokenIssuer struct {
	token string
}

func (i fakeTokenIssuer) IssueToken() (string, error) {
	return i.token, nil
}

type targetKey struct {
	tenantID   uint64
	examID     uint64
	targetType string
	targetID   uint64
}
