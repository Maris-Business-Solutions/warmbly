package models

import (
	"time"

	"github.com/google/uuid"
)

// AppListingStatus is where a directory listing can be seen.
type AppListingStatus string

const (
	// AppListingPublished is reachable by its link, and listed in discovery
	// once config.AppDirectoryPopularInstalls workspaces use it.
	AppListingPublished AppListingStatus = "published"
	// AppListingFeatured was picked by an operator and is always listed.
	AppListingFeatured AppListingStatus = "featured"
	// AppListingHidden was taken down by an operator and is reachable nowhere.
	AppListingHidden AppListingStatus = "hidden"
)

func (v AppListingStatus) Valid() bool {
	switch v {
	case AppListingPublished, AppListingFeatured, AppListingHidden:
		return true
	}
	return false
}

// AppListingCategories is the directory vocabulary: the built-in catalog's
// categories plus two that only community apps use.
var AppListingCategories = []string{
	string(IntegrationCategoryCRM),
	string(IntegrationCategoryAutomation),
	string(IntegrationCategoryNotifications),
	string(IntegrationCategoryMeetings),
	string(IntegrationCategoryData),
	string(IntegrationCategoryVerification),
	"ai",
	"other",
}

// AppListing is one OAuth app's page in the community directory.
type AppListing struct {
	ApplicationID  uuid.UUID        `json:"application_id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	Slug           string           `json:"slug"`
	Tagline        string           `json:"tagline"`
	Description    string           `json:"description"`
	Category       string           `json:"category"`
	InstallURL     string           `json:"install_url"`
	SupportURL     string           `json:"support_url"`
	PrivacyURL     string           `json:"privacy_url"`
	Status         AppListingStatus `json:"status"`
	StatusNote     string           `json:"status_note,omitempty"`
	StatusAt       *time.Time       `json:"status_at,omitempty"`
	SubmittedAt    time.Time        `json:"submitted_at"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// AppListingWrite is the developer's publish/edit payload.
type AppListingWrite struct {
	Slug        string `json:"slug"`
	Tagline     string `json:"tagline"`
	Description string `json:"description"`
	Category    string `json:"category"`
	InstallURL  string `json:"install_url"`
	SupportURL  string `json:"support_url"`
	PrivacyURL  string `json:"privacy_url"`
}

// CommunityApp is a listing as a workspace browsing the directory sees it:
// the app's public face, its publisher, its reach, and whether this workspace
// already uses it. No credential, redirect or webhook field is ever included.
type CommunityApp struct {
	ApplicationID uuid.UUID `json:"application_id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Tagline       string    `json:"tagline"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	LogoURL       string    `json:"logo_url"`
	WebsiteURL    string    `json:"website_url"`
	InstallURL    string    `json:"install_url"`
	SupportURL    string    `json:"support_url"`
	PrivacyURL    string    `json:"privacy_url"`
	Developer     string    `json:"developer"`
	Scopes        uint64    `json:"scopes"`
	// Permissions spells out Scopes for a viewer who cannot read the API
	// permission catalog.
	Permissions []APIPermission  `json:"permissions"`
	Status      AppListingStatus `json:"status"`
	// Listed: shown in discovery, because it is featured or widely used.
	Listed      bool      `json:"listed"`
	Installs    int       `json:"installs"`
	Installed   bool      `json:"installed"`
	PublishedAt time.Time `json:"published_at"`
}

// AdminAppListing is a listing in the operator's review queue.
type AdminAppListing struct {
	AppListing
	Name             string          `json:"name"`
	LogoURL          string          `json:"logo_url"`
	WebsiteURL       string          `json:"website_url"`
	Scopes           uint64          `json:"scopes"`
	Permissions      []APIPermission `json:"permissions"`
	AppStatus        string          `json:"app_status"`
	OrganizationName string          `json:"organization_name"`
	Installs         int             `json:"installs"`
	Listed           bool            `json:"listed"`
	StatusBy         *uuid.UUID      `json:"status_by,omitempty"`
	StatusByEmail    string          `json:"status_by_email,omitempty"`
}

// AdminAppListingSearch filters the review queue.
type AdminAppListingSearch struct {
	Q      string `form:"q"`
	Status string `form:"status"`
	Limit  int    `form:"limit"`
	Offset int    `form:"-"`
}

// AdminAppListingsResult is the review queue page.
type AdminAppListingsResult struct {
	Data       []AdminAppListing `json:"data"`
	Pagination Pagination        `json:"pagination"`
}

// SetAppListingStatus is an operator's decision on a listing; the note is
// shown to the developer and required to hide one.
type SetAppListingStatus struct {
	Status AppListingStatus `json:"status"`
	Note   string           `json:"note"`
}

// PermissionsIn lists the API permissions a scope mask grants, in catalog order.
func PermissionsIn(mask uint64) []APIPermission {
	out := []APIPermission{}
	for _, p := range AllAPIPermissions {
		if p.Value != 0 && mask&p.Value == p.Value {
			out = append(out, p)
		}
	}
	return out
}
