package notification

// DocumentCommentMention produces a minimal, privacy-preserving notification.
// The link rechecks resource access; mentioning someone never grants access.
func DocumentCommentMention(workspaceID, actorID, documentID, threadID uint, mentionedIDs []uint, publicBaseURL string) Intent {
	recipients := make([]uint, 0)
	for _, id := range dedupeUserIDs(mentionedIDs) {
		if id != actorID {
			recipients = append(recipients, id)
		}
	}
	vars := map[string]string{"url": DocumentCommentURL(publicBaseURL, documentID, threadID)}
	return Intent{WorkspaceID: workspaceID, DocumentID: documentID, ThreadID: threadID, Scene: SceneDocumentCommentMentioned,
		ActorUserID: actorID, RecipientUserIDs: recipients, Vars: vars,
		FallbackPlainText: PlainText(SceneDocumentCommentMentioned, vars)}
}
