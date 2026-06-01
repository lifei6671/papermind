import { useParams } from "react-router-dom";
import { QuestionCreatePage } from "./QuestionCreatePage";

type QuestionEditRouteProps = {
  tenantID: number;
  spaceID?: number;
};

export function QuestionEditRoute({ tenantID, spaceID }: QuestionEditRouteProps) {
  const params = useParams();
  const questionID = Number(params.questionID);

  return <QuestionCreatePage questionID={questionID} tenantID={tenantID} spaceID={spaceID} />;
}
