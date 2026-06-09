import { useEffect, useState } from "react";
import { Navigate, useLocation, useParams } from "react-router-dom";
import type { ExamManagementAPI } from "../../api/exams";
import { examApi } from "../../api/exams";
import type { ExamDetailAPI } from "../../api/examDetail";
import { examDetailApi } from "../../api/examDetail";
import type { PaperAPI } from "../../api/papers";
import type { PaperRow, PaperSectionRow } from "../../api/papers";
import type { QuestionAPI } from "../../api/questions";
import { paperApi } from "../../api/papers";
import { questionApi } from "../../api/questions";
import { StudentExamPage } from "../StudentExam/StudentExamPage";
import type { StudentExamPreviewData } from "../StudentExam/StudentExamPage";
import { PaperPreviewPage } from "./PaperPreviewPage";

type PaperPreviewRouteProps = {
  examDetailApi?: ExamDetailAPI;
  examApi?: ExamManagementAPI;
  paperApi?: PaperAPI;
  questionApi?: QuestionAPI;
  tenantID: number;
  spaceID?: number;
};

type LegacyPaperPreviewResolution =
  | { requestKey: string; status: "loading" }
  | { examID: number; requestKey: string; status: "canonical" }
  | { message: string; requestKey: string; status: "error" }
  | { requestKey: string; status: "paper" };

export function PaperPreviewRoute({
  examApi: providedExamApi = examApi,
  paperApi: providedPaperApi = paperApi,
  questionApi: providedQuestionApi = questionApi,
  tenantID,
  spaceID,
}: PaperPreviewRouteProps) {
  const params = useParams();
  const location = useLocation();
  const paperID = Number(params.paperID);
  const requestKey = `${tenantID}:${spaceID ?? "all"}:${paperID}`;
  const [resolution, setResolution] = useState<LegacyPaperPreviewResolution>({ requestKey, status: "loading" });
  const currentResolution = resolution.requestKey === requestKey ? resolution : { requestKey, status: "loading" as const };

  useEffect(() => {
    let ignore = false;

    async function resolveLegacyPreview() {
      const data = await providedExamApi.listExams({
        tenantID,
        ...(spaceID === undefined ? {} : { spaceID }),
        paperID,
        page: 1,
        pageSize: 2,
      });

      if (ignore) {
        return;
      }
      setResolution(data.total === 1 && data.items.length === 1
        ? { requestKey, status: "canonical", examID: data.items[0].id }
        : { requestKey, status: "paper" });
    }

    // 旧 paper 预览入口优先跳到唯一命中的考试详情；无法唯一映射时继续回退到旧试卷预览，
    // 避免草稿试卷或被多场考试复用的试卷失去预览入口。
    resolveLegacyPreview()
      .catch(() => {
        if (!ignore) {
          setResolution({ requestKey, status: "error", message: "考试详情定位失败，请稍后重试或联系管理员确认访问权限" });
        }
      });

    return () => {
      ignore = true;
    };
  }, [paperID, providedExamApi, requestKey, spaceID, tenantID]);

  const search = scopedPreviewSearch(location.search, spaceID);
  if (currentResolution.status === "loading") {
    return <div className="tenant-admin-status" role="status">正在定位考试详情</div>;
  }
  if (currentResolution.status === "canonical") {
    return <Navigate replace to={`/exams/${currentResolution.examID}${search}`} />;
  }
  if (currentResolution.status === "error") {
    return <div className="tenant-admin-status tenant-admin-status--error" role="alert">{currentResolution.message}</div>;
  }
  return (
    <PaperPreviewPage
      paperApi={providedPaperApi}
      paperID={paperID}
      questionApi={providedQuestionApi}
      tenantID={tenantID}
      spaceID={spaceID}
    />
  );
}

// ExamPreviewRoute 是考试详情页的 canonical 路由入口。
// 它只负责从 URL 读取 examID，真实数据加载和旧 paper 预览兼容逻辑仍收敛在 PaperPreviewPage。
export function ExamPreviewRoute({
  examDetailApi: providedExamDetailApi = examDetailApi,
  paperApi: providedPaperApi = paperApi,
  questionApi: providedQuestionApi = questionApi,
  tenantID,
  spaceID,
}: PaperPreviewRouteProps) {
  const params = useParams();
  const examID = Number(params.examID);

  return (
    <PaperPreviewPage
      examDetailApi={providedExamDetailApi}
      examID={examID}
      paperApi={providedPaperApi}
      questionApi={providedQuestionApi}
      tenantID={tenantID}
      spaceID={spaceID}
    />
  );
}

type StudentPreviewState =
  | { status: "loading" }
  | { message: string; status: "error" }
  | { preview: StudentExamPreviewData; status: "ready" };

export function PaperStudentPreviewRoute({
  paperApi: providedPaperApi = paperApi,
  tenantID,
  spaceID,
}: PaperPreviewRouteProps) {
  const params = useParams();
  const location = useLocation();
  const paperID = Number(params.paperID);
  const [state, setState] = useState<StudentPreviewState>({ status: "loading" });

  useEffect(() => {
    let ignore = false;

    async function loadStudentPreview() {
      try {
        const [paper, sectionData, sectionQuestionData] = await Promise.all([
          providedPaperApi.getPaper({ tenantID, paperID }),
          providedPaperApi.listSections({ tenantID, paperID }),
          providedPaperApi.listSectionQuestions({ tenantID, paperID }),
        ]);
        if (ignore) {
          return;
        }
        setState({
          status: "ready",
          preview: buildStudentPreviewData({
            exitPath: `/papers${scopedPreviewSearch(location.search, spaceID)}`,
            paper,
            sectionQuestions: sectionQuestionData.items,
            sections: sectionData.items,
          }),
        });
      } catch {
        if (!ignore) {
          setState({ status: "error", message: "学生视角预览加载失败，请稍后重试。" });
        }
      }
    }

    void loadStudentPreview();

    return () => {
      ignore = true;
    };
  }, [location.search, paperID, providedPaperApi, spaceID, tenantID]);

  if (state.status === "loading") {
    return <div className="tenant-admin-status" role="status">正在加载学生视角预览</div>;
  }
  if (state.status === "error") {
    return <div className="tenant-admin-status tenant-admin-status--error" role="alert">{state.message}</div>;
  }
  return <StudentExamPage preview={state.preview} tenantID={tenantID} />;
}

function scopedPreviewSearch(currentSearch: string, spaceID: number | undefined) {
  if (spaceID !== undefined) {
    return currentSearch;
  }
  const params = new URLSearchParams(currentSearch);
  params.delete("space_id");
  const query = params.toString();
  return query ? `?${query}` : "";
}

function buildStudentPreviewData({
  exitPath,
  paper,
  sectionQuestions,
  sections,
}: {
  exitPath: string;
  paper?: PaperRow;
  sectionQuestions: Awaited<ReturnType<PaperAPI["listSectionQuestions"]>>["items"];
  sections: PaperSectionRow[];
}): StudentExamPreviewData {
  const sectionByID = new Map(sections.map((section) => [section.id, section]));
  const sectionOrderByID = new Map(sections.map((section) => [section.id, section.sortOrder]));
  const orderedItems = [...sectionQuestions].sort((left, right) => {
    const sectionOrderDiff = (sectionOrderByID.get(left.sectionID) ?? 0) - (sectionOrderByID.get(right.sectionID) ?? 0);
    return sectionOrderDiff !== 0 ? sectionOrderDiff : left.sortOrder - right.sortOrder;
  });

  return {
    durationMinutes: paper?.durationMinutes,
    exitPath,
    title: paper?.name ?? "试卷预览",
    questions: orderedItems.map((item, index) => {
      const section = sectionByID.get(item.sectionID);
      if (!section || !item.questionType || !item.title) {
        throw new Error("paper student preview question data is incomplete");
      }
      return {
        id: item.questionID,
        number: index + 1,
        sectionTitle: section.name,
        sectionSubtitle: section.instructions || `第 ${index + 1} 题 / ${formatPreviewScore(item.score)} 分`,
        type: item.questionType,
        stem: item.title,
        score: Number(item.score || 0),
        options: (item.options ?? []).map((content, optionIndex) => ({
          id: optionIndex + 1,
          key: optionKey(optionIndex),
          content,
        })),
        blankCount: Math.max(item.blankCount ?? 1, 1),
      };
    }),
  };
}

function formatPreviewScore(score: string) {
  const numericScore = Number(score);
  return Number.isInteger(numericScore) ? String(numericScore) : String(Number(numericScore.toFixed(2)));
}

function optionKey(index: number) {
  if (index >= 0 && index < 26) {
    return String.fromCharCode(65 + index);
  }
  return String(index + 1);
}
