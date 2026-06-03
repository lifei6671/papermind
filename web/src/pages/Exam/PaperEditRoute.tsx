import { useParams } from "react-router-dom";
import type { PaperAPI } from "../../api/papers";
import type { QuestionAPI } from "../../api/questions";
import { paperApi } from "../../api/papers";
import { questionApi } from "../../api/questions";
import { PaperEditorPage } from "./PaperEditorPage";

type PaperEditRouteProps = {
  paperApi?: PaperAPI;
  questionApi?: QuestionAPI;
  tenantID: number;
  spaceID?: number;
};

export function PaperEditRoute({
  paperApi: providedPaperApi = paperApi,
  questionApi: providedQuestionApi = questionApi,
  tenantID,
  spaceID,
}: PaperEditRouteProps) {
  const params = useParams();
  const paperID = Number(params.paperID);

  return (
    <PaperEditorPage
      paperApi={providedPaperApi}
      questionApi={providedQuestionApi}
      paperID={paperID}
      tenantID={tenantID}
      spaceID={spaceID}
    />
  );
}
