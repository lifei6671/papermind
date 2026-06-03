import { useParams } from "react-router-dom";
import type { ActorRole } from "../../api/grading";
import { QuestionCreatePage } from "./QuestionCreatePage";

type QuestionEditRouteProps = {
  actorRole?: ActorRole;
  tenantID: number;
  spaceID?: number;
};

export function QuestionEditRoute({ actorRole, tenantID, spaceID }: QuestionEditRouteProps) {
  const params = useParams();
  const questionID = Number(params.questionID);

  return <QuestionCreatePage actorRole={actorRole} questionID={questionID} tenantID={tenantID} spaceID={spaceID} />;
}
