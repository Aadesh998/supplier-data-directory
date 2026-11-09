// Types mirroring packages/openapi/openapi.yaml. Keep in sync with the contract.

export type ApiErrorBody = {
  error: {
    code: string;
    message: string;
    request_id: string;
    details?: {
      fields?: Record<string, string>;
      reasons?: string[];
      [key: string]: unknown;
    };
  };
};

export type PageInfo = {
  page: number;
  limit: number;
  total: number;
  has_more: boolean;
};

export type Paginated<T> = { data: T[]; page: PageInfo };
export type DataList<T> = { data: T[] };

export type VerificationStatus = "unverified" | "partially_verified" | "verified" | "stale";

export type CategoryRef = { id?: string; slug: string; name: string };

export type Category = {
  id: string;
  parent_id: string | null;
  name: string;
  slug: string;
  category_type: "industry" | "product_category" | "service" | "trade";
  description?: string;
  sort_order?: number;
  active?: boolean;
  published_company_count?: number;
};

export type CategoryNode = Category & { children?: CategoryNode[] };

export type CategoryDetail = {
  category: Category;
  ancestors: CategoryRef[];
  children: Category[];
};

export type State = {
  code: string;
  name: string;
  slug: string;
  region_type: "state" | "union_territory";
  published_company_count?: number;
};

export type Role = { code: string; name: string; description?: string };

export type PublicCompanySummary = {
  slug: string;
  display_name: string;
  description?: string;
  city?: string;
  state_code?: string;
  state_name?: string;
  roles?: string[];
  categories?: CategoryRef[];
  verification_status?: VerificationStatus;
  updated_at?: string;
};

export type CertificationStatus = "claimed" | "unverified" | "verified" | "expired" | "revoked" | "rejected";

export type PublicCompany = PublicCompanySummary & {
  legal_name?: string;
  website_url?: string;
  establishment_year?: number;
  facilities?: {
    facility_type?: string;
    locality?: string;
    city?: string;
    district?: string;
    state_code?: string;
    state_name?: string;
  }[];
  products?: { name: string; description?: string; category?: CategoryRef }[];
  capabilities?: { slug: string; name: string; verification_status?: string }[];
  certifications?: { certification_type: string; issuer?: string; status?: CertificationStatus }[];
  contacts?: {
    contact_type: "public_business_email" | "switchboard" | "sales_department";
    channel: "email" | "phone" | "url";
    label?: string;
    value: string;
  }[];
  source_labels?: string[];
  published_at?: string;
  last_verified_at?: string;
  /** Details that exist but are withheld from the open web (counts/flags only, never values). */
  locked?: LockedDetails;
};

export type LockedDetails = {
  contacts: number;
  website: boolean;
  legal_name: boolean;
  facility_details: number;
  certification_issuers: number;
  more_products: number;
};

export type SitemapData = {
  companies?: { slug: string; updated_at?: string }[];
  categories?: { slug: string; updated_at?: string }[];
  states?: { slug: string }[];
};

export type FacilityType =
  | "factory"
  | "office"
  | "registered_office"
  | "warehouse"
  | "showroom"
  | "workshop"
  | "laboratory"
  | "other";

export type SupplierSubmissionRequest = {
  company: {
    display_name: string;
    legal_name?: string;
    description?: string;
    website_url?: string;
    establishment_year?: number;
  };
  roles: string[];
  categories: string[];
  products?: { name: string; description?: string; unit?: string }[];
  facilities: {
    facility_type: FacilityType;
    address_line1?: string;
    locality?: string;
    city: string;
    district?: string;
    state_code: string;
    postal_code?: string;
  }[];
  business_contacts?: {
    contact_type: "public_business_email" | "switchboard" | "sales_department";
    channel: "email" | "phone";
    value: string;
  }[];
  submitter: { name: string; email: string; designation?: string; phone?: string };
  consents: { privacy_notice_acknowledged: boolean; public_listing: boolean; commercial_api: boolean };
  privacy_notice_version: string;
  form_started_at: number;
  website_confirm?: string;
};

export type AcceptedResponse = { id: string; status: string; message?: string };

export type PrivacyRequestType =
  | "correction"
  | "removal"
  | "withdraw_public_listing"
  | "withdraw_commercial_api"
  | "access"
  | "other";

export type PrivacyRequest = {
  request_type: PrivacyRequestType;
  requester_name: string;
  email: string;
  company_reference?: string;
  details?: string;
  website_confirm?: string;
};

// ---------- Admin ----------

export type AdminRole = "owner" | "admin" | "reviewer";

export type AdminUser = {
  id: string;
  email: string;
  display_name?: string;
  role: AdminRole;
  active?: boolean;
  last_login_at?: string | null;
};

export type Session = { user: AdminUser; csrf_token: string };

export type SourceType =
  | "self_submission"
  | "authorized_import"
  | "official_site"
  | "licensed_feed"
  | "permitted_api_lookup"
  | "manual_verification"
  | "public_web_listing";

export type DataSource = {
  id: string;
  source_type: SourceType;
  provider: string;
  source_url: string | null;
  license_reference: string | null;
  collected_at?: string;
  allowed_internal_use: boolean;
  allowed_public_display: boolean;
  allowed_commercial_api: boolean;
  retention_expires_at: string | null;
  notes: string | null;
  created_at?: string;
};

export type DataSourceInput = {
  source_type: string;
  provider: string;
  source_url?: string;
  license_reference?: string;
  allowed_internal_use?: boolean;
  allowed_public_display?: boolean;
  allowed_commercial_api?: boolean;
  retention_expires_at?: string;
  notes?: string;
};

export type PublicationStatus =
  | "draft"
  | "submitted"
  | "under_review"
  | "approved_private"
  | "published"
  | "rejected"
  | "needs_changes"
  | "archived";

export type AdminCompanySummary = {
  id: string;
  display_name: string;
  legal_name: string | null;
  slug: string;
  publication_status: PublicationStatus;
  verification_status: VerificationStatus;
  city: string | null;
  state_code: string | null;
  website_domain: string | null;
  roles: string[];
  categories: CategoryRef[];
  created_at: string;
  updated_at: string;
};

export type Facility = {
  id: string;
  facility_type: string;
  name: string | null;
  address_line1: string | null;
  address_line2: string | null;
  locality: string | null;
  city: string | null;
  district: string | null;
  state_code: string | null;
  postal_code: string | null;
  is_primary: boolean;
  verification_status: string;
  source_id: string | null;
};

export type Product = {
  id: string;
  name: string;
  description: string | null;
  unit: string | null;
  category: CategoryRef | null;
  source_id: string | null;
};

export type Contact = {
  id: string;
  contact_type: string;
  channel: string;
  label: string | null;
  value: string;
  masked: boolean;
  is_public: boolean;
  lawful_basis_or_permission_ref: string | null;
  source_id: string | null;
};

export type Provenance = {
  entity_type: string;
  entity_id: string;
  field_name: string;
  permission_status: "unknown" | "permitted" | "restricted" | "withdrawn";
  observed_at: string;
  source?: DataSource;
};

export type VerificationEvent = {
  id: string;
  field_name: string | null;
  method: string;
  result: string;
  notes: string | null;
  checked_at: string;
  checked_by: string;
  next_review_at: string | null;
};

export type AdminCompany = AdminCompanySummary & {
  normalized_name?: string;
  description: string | null;
  website_url: string | null;
  establishment_year: number | null;
  business_status?: "active" | "inactive" | "closed" | "unknown";
  primary_category_id: string | null;
  published_at: string | null;
  last_verified_at: string | null;
  next_review_at: string | null;
  facilities?: Facility[];
  products?: Product[];
  capabilities?: { id: string; slug: string; name: string; verification_status: string }[];
  certifications?: { id: string; certification_type: string; issuer: string | null; status: string }[];
  contacts?: Contact[];
  provenance?: Provenance[];
  verification_events?: VerificationEvent[];
  publish_check?: { allowed: boolean; reasons: string[] };
};

export type CompanyInput = {
  source_id?: string;
  display_name?: string;
  legal_name?: string;
  description?: string;
  website_url?: string;
  establishment_year?: number;
  business_status?: string;
  primary_category_id?: string;
  slug?: string;
  roles?: string[];
  category_ids?: string[];
  publication_status?: string;
};

export type SubmissionStatus =
  | "submitted"
  | "under_review"
  | "approved"
  | "rejected"
  | "needs_changes"
  | "spam"
  | "withdrawn";

export type Submission = {
  id: string;
  status: SubmissionStatus;
  company_name: string;
  submitted_at: string;
  reviewed_at: string | null;
  privacy_notice_version: string;
  consents: {
    storage_acknowledged_at?: string;
    public_listing_at: string | null;
    commercial_api_at: string | null;
  };
  company_id: string | null;
};

export type SubmissionDetail = Submission & {
  payload: SupplierSubmissionRequest;
  submitter_email?: string;
  review_notes: string | null;
};

export type ImportJob = {
  id: string;
  filename: string;
  status: "uploaded" | "staged" | "committed" | "failed" | "rolled_back" | "cancelled";
  headers?: string[];
  column_mapping?: Record<string, string>;
  total_rows: number;
  valid_rows: number;
  error_rows: number;
  duplicate_rows: number;
  committed_rows: number;
  source?: DataSource;
  created_at: string;
  committed_at: string | null;
  /** Background task currently running (null when idle). */
  task?: "process" | "dry_run" | "commit" | null;
  task_started_at?: string | null;
  task_finished_at?: string | null;
  /** Last task's result: CommitResult for dry_run/commit, row counts for process. */
  task_result?: (CommitResult & Record<string, unknown>) | Record<string, unknown> | null;
  task_error?: string | null;
};

export type FieldMessage = { field: string; message: string };

export type ImportRow = {
  id: string;
  row_number: number;
  raw_data: Record<string, string>;
  normalized_data?: Record<string, unknown>;
  transformation_log?: string[];
  validation_errors?: FieldMessage[];
  validation_warnings?: FieldMessage[];
  duplicate_candidates?: { company_id: string; display_name: string; score: number; reasons?: string[] }[];
  disposition: "pending" | "accepted" | "rejected" | "merged";
  matched_company_id: string | null;
  created_company_id: string | null;
};

export type ImportField = { field: string; label: string; required: boolean; description?: string };

export type CommitResult = {
  dry_run: boolean;
  created: number;
  merged: number;
  skipped: number;
  rejected: number;
  errors?: { row_number: number; message: string }[];
};

export type DuplicatePair = {
  id: string;
  score: number;
  reasons: string[];
  status: "open" | "merged" | "dismissed";
  company_a: AdminCompanySummary;
  company_b: AdminCompanySummary;
  created_at: string;
};

export type AuditLog = {
  id: string;
  actor_email: string | null;
  actor_type: string;
  action: string;
  entity_type: string;
  entity_id: string | null;
  before: Record<string, unknown> | null;
  after: Record<string, unknown> | null;
  request_id: string | null;
  created_at: string;
};

export type PrivacyRequestRecord = {
  id: string;
  request_type: string;
  requester: string;
  email: string;
  company_id: string | null;
  company_reference: string | null;
  details: string | null;
  status: "received" | "in_progress" | "resolved" | "rejected";
  received_at: string;
  resolved_at: string | null;
  resolution_notes: string | null;
};

export type Stats = {
  companies_by_publication_status: Record<string, number>;
  companies_by_verification_status: Record<string, number>;
  submissions_pending: number;
  imports_staged: number;
  open_duplicates: number;
  privacy_requests_open: number;
  due_for_review: number;
};

export type DiscoveryStatus = { enabled: boolean; provider?: string; reason?: string };

export type DiscoveryResult = {
  provider_place_id: string;
  display_name: string;
  formatted_address?: string;
  google_maps_uri?: string;
};

export type DiscoverySearchResponse = {
  results: DiscoveryResult[];
  attribution?: string;
  notice?: string;
};

export type DiscoveryLead = {
  id: string;
  provider: string;
  provider_place_id: string;
  search_query: string | null;
  status: "new" | "invited" | "registered" | "dismissed";
  notes: string | null;
  created_at: string;
};

// --- Discovery job queue ---
export type DiscoveryProvider = "osm_overpass" | "google_places_ids" | "gmaps_scraper";

export type DiscoveryJob = {
  id: string;
  job_key: string;
  provider: DiscoveryProvider;
  priority: number;
  sector_family: string | null;
  category_slug: string | null;
  keyword: string;
  search_query: string;
  location: string;
  state_code: string | null;
  radius_km: number;
  is_cluster: boolean;
  status: "pending" | "running" | "done" | "failed" | "paused";
  attempts: number;
  max_attempts: number;
  refresh_interval_days: number;
  locked_by: string | null;
  last_run_at: string | null;
  next_run_after: string | null;
  last_error: string | null;
  results_count: number | null;
  total_results: number;
};

export type JobBatch = {
  id: string;
  filename: string;
  inserted: number;
  updated: number;
  skipped: number;
  errors: string[];
  created_at: string;
};

export type JobStats = {
  by_status: Record<string, number>;
  due_now: number;
  results_by_status: Record<string, number>;
  last_batch: JobBatch | null;
  last_run_at: string | null;
  total_results: number;
};

export type DiscoveryJobResult = {
  id: string;
  provider: DiscoveryProvider;
  provider_ref: string;
  category_slug: string | null;
  name: string | null;
  latitude: number | null;
  longitude: number | null;
  tags: Record<string, string>;
  status: "new" | "promoted" | "contacted" | "dismissed";
  matched_company_id: string | null;
  seen_count: number;
  job_location: string | null;
  job_state_code: string | null;
  first_seen_at: string;
  last_seen_at: string;
};

/** A business found by the Google Maps worker (gmaps_leads). */
export type GmapsLead = {
  id: string;
  title: string;
  category: string | null;
  address: string | null;
  phone: string | null;
  website: string | null;
  emails: string | null;
  review_count: number | null;
  review_rating: number | null;
  business_status: string | null;
  maps_link: string | null;
  category_slug: string | null;
  state_code: string | null;
  location: string | null;
  search_query: string | null;
  status: "new" | "promoted" | "contacted" | "dismissed";
  promoted_company_id: string | null;
  seen_count: number;
  first_seen_at: string;
  last_seen_at: string;
};
