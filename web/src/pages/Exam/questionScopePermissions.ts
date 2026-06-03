import type { ActorRole } from "../../api/grading";

export function canWritePublicQuestionScope(actorRole?: ActorRole) {
  return actorRole === undefined || actorRole === "tenant_admin" || actorRole === "teacher";
}
