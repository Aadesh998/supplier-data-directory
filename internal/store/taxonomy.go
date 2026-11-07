package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bharatmfg/directory/internal/normalize"
)

// Category mirrors categories with a published company count.
type Category struct {
	ID                    string     `json:"id"`
	ParentID              *string    `json:"parent_id"`
	Name                  string     `json:"name"`
	Slug                  string     `json:"slug"`
	CategoryType          string     `json:"category_type"`
	Description           string     `json:"description"`
	SortOrder             int        `json:"sort_order"`
	Active                bool       `json:"active"`
	PublishedCompanyCount int        `json:"published_company_count"`
	Children              []Category `json:"children,omitempty"`
}

// ListCategories returns categories with published counts (including descendants).
func ListCategories(ctx context.Context, q DBTX, includeInactive bool) ([]Category, error) {
	activeFilter := "WHERE cat.active"
	if includeInactive {
		activeFilter = ""
	}
	return collect(ctx, q, `WITH RECURSIVE tree AS (
			SELECT id, id AS root FROM categories
			UNION ALL SELECT ch.id, t.root FROM categories ch JOIN tree t ON ch.parent_id = t.id),
		counts AS (
			SELECT t.root, count(DISTINCT c.id) AS n FROM tree t
			JOIN company_categories cc ON cc.category_id = t.id
			JOIN companies c ON c.id = cc.company_id AND `+publishedWhere+`
			WHERE EXISTS (SELECT 1 FROM data_sources ds WHERE ds.id = cc.source_id AND ds.allowed_public_display)
			GROUP BY t.root)
		SELECT cat.id, cat.parent_id, cat.name, cat.slug, cat.category_type, cat.description, cat.sort_order, cat.active,
		       COALESCE(counts.n, 0)
		FROM categories cat LEFT JOIN counts ON counts.root = cat.id `+activeFilter+`
		ORDER BY cat.sort_order, cat.name`, func(r pgx.Rows) (Category, error) {
		var c Category
		return c, r.Scan(&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.CategoryType, &c.Description, &c.SortOrder, &c.Active, &c.PublishedCompanyCount)
	})
}

// BuildTree nests a flat category list.
func BuildTree(flat []Category) []Category {
	children := map[string][]Category{}
	var roots []Category
	for _, c := range flat {
		if c.ParentID == nil {
			roots = append(roots, c)
		} else {
			children[*c.ParentID] = append(children[*c.ParentID], c)
		}
	}
	var attach func(c Category) Category
	attach = func(c Category) Category {
		for _, ch := range children[c.ID] {
			c.Children = append(c.Children, attach(ch))
		}
		if c.Children == nil {
			c.Children = []Category{}
		}
		return c
	}
	out := make([]Category, 0, len(roots))
	for _, r := range roots {
		out = append(out, attach(r))
	}
	return out
}

// CategoryDetail is a category with its ancestors and children.
type CategoryDetail struct {
	Category  Category      `json:"category"`
	Ancestors []CategoryRef `json:"ancestors"`
	Children  []Category    `json:"children"`
}

// GetCategoryDetail loads an active category by slug.
func GetCategoryDetail(ctx context.Context, q DBTX, slug string) (CategoryDetail, error) {
	all, err := ListCategories(ctx, q, false)
	if err != nil {
		return CategoryDetail{}, err
	}
	byID := map[string]Category{}
	var found *Category
	for i := range all {
		byID[all[i].ID] = all[i]
		if all[i].Slug == slug {
			found = &all[i]
		}
	}
	if found == nil {
		return CategoryDetail{}, pgx.ErrNoRows
	}
	d := CategoryDetail{Category: *found, Ancestors: []CategoryRef{}, Children: []Category{}}
	for p := found.ParentID; p != nil; {
		parent, ok := byID[*p]
		if !ok {
			break
		}
		d.Ancestors = append([]CategoryRef{{ID: parent.ID, Slug: parent.Slug, Name: parent.Name}}, d.Ancestors...)
		p = parent.ParentID
	}
	for _, c := range all {
		if c.ParentID != nil && *c.ParentID == found.ID {
			d.Children = append(d.Children, c)
		}
	}
	return d, nil
}

// CategoryInput creates or updates a category.
type CategoryInput struct {
	ParentID     *string `json:"parent_id" validate:"omitempty,uuid"`
	Name         *string `json:"name" validate:"omitempty,min=1,max=200"`
	Slug         *string `json:"slug" validate:"omitempty,max=160"`
	CategoryType *string `json:"category_type" validate:"omitempty,oneof=industry product_category service trade"`
	Description  *string `json:"description" validate:"omitempty,max=5000"`
	Active       *bool   `json:"active"`
	SortOrder    *int    `json:"sort_order"`
}

func getCategory(ctx context.Context, q DBTX, id string) (Category, error) {
	var c Category
	err := q.QueryRow(ctx, `SELECT id, parent_id, name, slug, category_type, description, sort_order, active FROM categories WHERE id = $1`, id).
		Scan(&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.CategoryType, &c.Description, &c.SortOrder, &c.Active)
	return c, err
}

// CreateCategory adds a category.
func (s *Store) CreateCategory(ctx context.Context, a Actor, in CategoryInput) (Category, error) {
	if in.Name == nil || strings.TrimSpace(*in.Name) == "" {
		return Category{}, ErrInvalid{Msg: "name required", Fields: map[string]string{"name": "is required"}}
	}
	slug := normalize.Slug(*in.Name)
	if in.Slug != nil && *in.Slug != "" {
		slug = normalize.Slug(*in.Slug)
	}
	typ := "product_category"
	if in.CategoryType != nil {
		typ = *in.CategoryType
	}
	var c Category
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO categories (parent_id, name, slug, category_type, description, sort_order)
			VALUES ($1,$2,$3,$4,COALESCE($5,''),COALESCE($6,0)) RETURNING id`, in.ParentID, normalize.Whitespace(*in.Name), slug, typ,
			in.Description, in.SortOrder).Scan(&id); err != nil {
			return err
		}
		var err error
		if c, err = getCategory(ctx, tx, id); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "category.create", "category", &id, nil, c)
	})
	return c, err
}

// UpdateCategory edits a category (slug changes are not allowed to keep URLs stable).
func (s *Store) UpdateCategory(ctx context.Context, a Actor, id string, in CategoryInput) (Category, error) {
	var c Category
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		before, err := getCategory(ctx, tx, id)
		if err != nil {
			return err
		}
		if in.Slug != nil && *in.Slug != before.Slug {
			return ErrInvalid{Msg: "slug cannot be changed", Fields: map[string]string{"slug": "cannot be changed once created (public URLs depend on it)"}}
		}
		if _, err := tx.Exec(ctx, `UPDATE categories SET parent_id = CASE WHEN $2 THEN $3::uuid ELSE parent_id END,
			name = COALESCE($4, name), category_type = COALESCE($5, category_type), description = COALESCE($6, description),
			active = COALESCE($7, active), sort_order = COALESCE($8, sort_order) WHERE id = $1`,
			id, in.ParentID != nil, in.ParentID, in.Name, in.CategoryType, in.Description, in.Active, in.SortOrder); err != nil {
			return err
		}
		if c, err = getCategory(ctx, tx, id); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "category.update", "category", &id, before, c)
	})
	return c, err
}

// State is an Indian state/UT with its published count.
type State struct {
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	Slug                  string `json:"slug"`
	RegionType            string `json:"region_type"`
	PublishedCompanyCount int    `json:"published_company_count"`
}

// ListStates returns states/UTs with published company counts.
func ListStates(ctx context.Context, q DBTX) ([]State, error) {
	return collect(ctx, q, `SELECT s.code, s.name, s.slug, s.region_type, count(DISTINCT c.id)
		FROM indian_states s
		LEFT JOIN facilities f ON f.state_code = s.code AND f.deleted_at IS NULL
		  AND EXISTS (SELECT 1 FROM public_field_permissions pf WHERE pf.entity_type = 'facility' AND pf.entity_id = f.id AND pf.field_name = 'state_code')
		LEFT JOIN companies c ON c.id = f.company_id AND `+publishedWhere+`
		GROUP BY s.code ORDER BY s.name`, func(r pgx.Rows) (State, error) {
		var s State
		return s, r.Scan(&s.Code, &s.Name, &s.Slug, &s.RegionType, &s.PublishedCompanyCount)
	})
}

// Role is a business role.
type Role struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListRoles returns business roles.
func ListRoles(ctx context.Context, q DBTX) ([]Role, error) {
	return collect(ctx, q, `SELECT code, name, description FROM business_roles ORDER BY sort_order`, func(r pgx.Rows) (Role, error) {
		var x Role
		return x, r.Scan(&x.Code, &x.Name, &x.Description)
	})
}

// Stats summarises the admin dashboard.
type Stats struct {
	ByPublication       map[string]int `json:"companies_by_publication_status"`
	ByVerification      map[string]int `json:"companies_by_verification_status"`
	SubmissionsPending  int            `json:"submissions_pending"`
	ImportsStaged       int            `json:"imports_staged"`
	OpenDuplicates      int            `json:"open_duplicates"`
	PrivacyRequestsOpen int            `json:"privacy_requests_open"`
	DueForReview        int            `json:"due_for_review"`
	GeneratedAt         time.Time      `json:"generated_at"`
}

// GetStats computes dashboard counters.
func GetStats(ctx context.Context, q DBTX) (Stats, error) {
	st := Stats{ByPublication: map[string]int{}, ByVerification: map[string]int{}, GeneratedAt: time.Now()}
	for col, m := range map[string]map[string]int{"publication_status": st.ByPublication, "verification_status": st.ByVerification} {
		rows, err := collect(ctx, q, fmt.Sprintf(`SELECT %s, count(*) FROM companies WHERE deleted_at IS NULL GROUP BY 1`, col),
			func(r pgx.Rows) (struct {
				k string
				n int
			}, error) {
				var x struct {
					k string
					n int
				}
				return x, r.Scan(&x.k, &x.n)
			})
		if err != nil {
			return st, err
		}
		for _, r := range rows {
			m[r.k] = r.n
		}
	}
	err := q.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM supplier_submissions WHERE status IN ('submitted', 'under_review')),
		(SELECT count(*) FROM import_jobs WHERE status = 'staged'),
		(SELECT count(*) FROM duplicate_pairs WHERE status = 'open'),
		(SELECT count(*) FROM deletion_requests WHERE status IN ('received', 'in_progress')),
		(SELECT count(*) FROM companies WHERE deleted_at IS NULL AND next_review_at < now())`).
		Scan(&st.SubmissionsPending, &st.ImportsStaged, &st.OpenDuplicates, &st.PrivacyRequestsOpen, &st.DueForReview)
	return st, err
}
