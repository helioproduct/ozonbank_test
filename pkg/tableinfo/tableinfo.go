package tableinfo

const (
	PostsTableName = "posts"

	PostIDColumn              = "id"
	PostTitleColumn           = "title"
	PostBodyColumn            = "body"
	PostUserIDColumn          = "user_id"
	PostCommentsEnabledColumn = "comments_enabled"
	PostCreatedAtColumn       = "created_at"
)

const (
	CommentsTableName = "comments"

	CommentIDColumn        = "id"
	CommentPostIDColumn    = "post_id"
	CommentParentIDColumn  = "parent_id"
	CommentUserIDColumn    = "user_id"
	CommentBodyColumn      = "body"
	CommentCreatedAtColumn = "created_at"
)
