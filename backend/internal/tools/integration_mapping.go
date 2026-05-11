package tools

// ToolIntegrationMap maps tool names to their corresponding integration types.
// This enables automatic credential injection based on the tool being called.
// When a tool is executed, we look up its integration type and find a matching
// credential from the user's configured credentials.
var ToolIntegrationMap = map[string]string{
	// Communication tools
	"send_discord_message":     "discord",
	"send_slack_message":       "slack",
	"send_telegram_message":    "telegram",
	"send_teams_message":       "teams",
	"send_google_chat_message": "google_chat",
	"zoom_meeting":             "zoom",
	"twilio_send_sms":          "twilio",
	"twilio_send_whatsapp":     "twilio",
	"referralmonk_whatsapp":    "referralmonk",

	// Unipile (WhatsApp + LinkedIn messaging)
	"unipile_list_accounts":            "unipile",
	"unipile_list_attendees":           "unipile",
	"unipile_whatsapp_send_message":    "unipile",
	"unipile_whatsapp_send_to_phone":   "unipile",
	"unipile_whatsapp_list_chats":      "unipile",
	"unipile_whatsapp_get_messages":    "unipile",
	"unipile_linkedin_send_message":    "unipile",
	"unipile_linkedin_list_chats":      "unipile",
	"unipile_linkedin_get_messages":    "unipile",
	"unipile_linkedin_search_profiles": "unipile",

	// Email tools
	"send_email":       "sendgrid",
	"send_brevo_email": "brevo",

	// Generic webhook
	"send_webhook": "custom_webhook",

	// REST API
	"api_request": "rest_api",

	// Notion tools
	"notion_search":         "notion",
	"notion_query_database": "notion",
	"notion_create_page":    "notion",
	"notion_update_page":    "notion",

	// GitHub tools
	"github_create_issue": "github",
	"github_list_issues":  "github",
	"github_get_repo":     "github",
	"github_add_comment":  "github",

	// GitLab tools
	"gitlab_projects": "gitlab",
	"gitlab_issues":   "gitlab",
	"gitlab_mrs":      "gitlab",

	// Linear tools
	"linear_issues":       "linear",
	"linear_create_issue": "linear",
	"linear_update_issue": "linear",

	// Jira tools
	"jira_issues":       "jira",
	"jira_create_issue": "jira",
	"jira_update_issue": "jira",

	// Productivity tools
	"clickup_tasks":        "clickup",
	"clickup_create_task":  "clickup",
	"clickup_update_task":  "clickup",
	"calendly_events":      "calendly",
	"calendly_event_types": "calendly",
	"calendly_invitees":    "calendly",

	// Airtable tools
	"airtable_list":   "airtable",
	"airtable_read":   "airtable",
	"airtable_create": "airtable",
	"airtable_update": "airtable",

	// Trello tools
	"trello_boards":      "trello",
	"trello_lists":       "trello",
	"trello_cards":       "trello",
	"trello_create_card": "trello",

	// CRM tools
	"leadsquared_leads":       "leadsquared",
	"leadsquared_create_lead": "leadsquared",
	"leadsquared_activities":  "leadsquared",
	"hubspot_contacts":        "hubspot",
	"hubspot_deals":           "hubspot",
	"hubspot_companies":       "hubspot",

	// Marketing tools
	"mailchimp_lists":          "mailchimp",
	"mailchimp_add_subscriber": "mailchimp",

	// Analytics tools
	"mixpanel_track":        "mixpanel",
	"mixpanel_user_profile": "mixpanel",
	"posthog_capture":       "posthog",
	"posthog_identify":      "posthog",
	"posthog_query":         "posthog",

	// E-commerce tools
	"shopify_products":  "shopify",
	"shopify_orders":    "shopify",
	"shopify_customers": "shopify",

	// Deployment tools
	"netlify_sites":         "netlify",
	"netlify_deploys":       "netlify",
	"netlify_trigger_build": "netlify",

	// Storage tools
	"s3_list":     "aws_s3",
	"s3_upload":   "aws_s3",
	"s3_download": "aws_s3",
	"s3_delete":   "aws_s3",

	// Social Media tools
	"x_search_posts":   "x_twitter",
	"x_post_tweet":     "x_twitter",
	"x_get_user":       "x_twitter",
	"x_get_user_posts": "x_twitter",

	// Database tools
	"mongodb_query": "mongodb",
	"mongodb_write": "mongodb",
	"redis_read":    "redis",
	"redis_write":   "redis",

	// Composio Google Sheets tools
	"googlesheets_read":         "composio_googlesheets",
	"googlesheets_write":        "composio_googlesheets",
	"googlesheets_append":       "composio_googlesheets",
	"googlesheets_create":       "composio_googlesheets",
	"googlesheets_get_info":     "composio_googlesheets",
	"googlesheets_list_sheets":  "composio_googlesheets",
	"googlesheets_search":       "composio_googlesheets",
	"googlesheets_clear":        "composio_googlesheets",
	"googlesheets_add_sheet":    "composio_googlesheets",
	"googlesheets_delete_sheet": "composio_googlesheets",
	"googlesheets_find_replace": "composio_googlesheets",
	"googlesheets_upsert_rows":  "composio_googlesheets",

	// Composio Gmail tools
	"gmail_send_email":      "composio_gmail",
	"gmail_fetch_emails":    "composio_gmail",
	"gmail_get_message":     "composio_gmail",
	"gmail_reply_to_thread": "composio_gmail",
	"gmail_create_draft":    "composio_gmail",
	"gmail_send_draft":      "composio_gmail",
	"gmail_list_drafts":     "composio_gmail",
	"gmail_add_label":       "composio_gmail",
	"gmail_list_labels":     "composio_gmail",
	"gmail_move_to_trash":   "composio_gmail",

	// Composio LinkedIn tools
	"linkedin_create_comment":        "composio_linkedin",
	"linkedin_create_post":           "composio_linkedin",
	"linkedin_delete_post":           "composio_linkedin",
	"linkedin_delete_ugc_post":       "composio_linkedin",
	"linkedin_delete_ugc_posts":      "composio_linkedin",
	"linkedin_get_company_info":      "composio_linkedin",
	"linkedin_get_images":            "composio_linkedin",
	"linkedin_get_my_info":           "composio_linkedin",
	"linkedin_get_video":             "composio_linkedin",
	"linkedin_get_videos":            "composio_linkedin",
	"linkedin_register_image_upload": "composio_linkedin",

	// Composio Google Calendar tools
	"googlecalendar_create_event":    "composio_googlecalendar",
	"googlecalendar_list_events":     "composio_googlecalendar",
	"googlecalendar_find_free_slots": "composio_googlecalendar",
	"googlecalendar_quick_add_event": "composio_googlecalendar",
	"googlecalendar_delete_event":    "composio_googlecalendar",
	"googlecalendar_get_event":       "composio_googlecalendar",
	"googlecalendar_update_event":    "composio_googlecalendar",
	"googlecalendar_list_calendars":  "composio_googlecalendar",

	// Composio Google Drive tools
	"googledrive_list_files":    "composio_googledrive",
	"googledrive_get_file":      "composio_googledrive",
	"googledrive_create_folder": "composio_googledrive",
	"googledrive_search_files":  "composio_googledrive",
	"googledrive_delete_file":   "composio_googledrive",
	"googledrive_copy_file":     "composio_googledrive",
	"googledrive_move_file":     "composio_googledrive",
	"googledrive_download_file": "composio_googledrive",

	// Composio Canva tools
	"canva_list_designs":         "composio_canva",
	"canva_get_design":           "composio_canva",
	"canva_create_design":        "composio_canva",
	"canva_export_design":        "composio_canva",
	"canva_list_brand_templates": "composio_canva",
	"canva_upload_asset":         "composio_canva",

	// Composio Twitter tools
	"twitter_post_tweet":    "composio_twitter",
	"twitter_get_timeline":  "composio_twitter",
	"twitter_search_tweets": "composio_twitter",
	"twitter_get_user":      "composio_twitter",
	"twitter_get_me":        "composio_twitter",
	"twitter_like_tweet":    "composio_twitter",
	"twitter_retweet":       "composio_twitter",
	"twitter_delete_tweet":  "composio_twitter",

	// Composio YouTube tools
	"youtube_search_videos":      "composio_youtube",
	"youtube_get_video":          "composio_youtube",
	"youtube_get_channel":        "composio_youtube",
	"youtube_get_my_channel":     "composio_youtube",
	"youtube_list_playlists":     "composio_youtube",
	"youtube_get_playlist_items": "composio_youtube",
	"youtube_get_comments":       "composio_youtube",
	"youtube_subscribe":          "composio_youtube",

	// Composio Zoom tools
	"zoom_create_meeting": "composio_zoom",
	"zoom_list_meetings":  "composio_zoom",
	"zoom_get_meeting":    "composio_zoom",
	"zoom_update_meeting": "composio_zoom",
	"zoom_delete_meeting": "composio_zoom",
	"zoom_get_user":       "composio_zoom",
	"zoom_add_registrant": "composio_zoom",
}

// GetIntegrationTypeForTool returns the integration type for a given tool name.
// Returns empty string if the tool doesn't require credentials.
func GetIntegrationTypeForTool(toolName string) string {
	return ToolIntegrationMap[toolName]
}

// ToolRequiresCredential returns true if the tool requires a credential.
func ToolRequiresCredential(toolName string) bool {
	_, exists := ToolIntegrationMap[toolName]
	return exists
}
