package types

type MediaType string

const MediaTypeYouTubeVideo MediaType = "yt_video"
const MediaTypeSoundCloudTrack MediaType = "sc_track"
const MediaTypeDocument MediaType = "document"

type MediaCollectionType string

const MediaCollectionTypeYouTubeChannel MediaCollectionType = "yt_channel"
const MediaCollectionTypeSoundCloudUser MediaCollectionType = "sc_user"
