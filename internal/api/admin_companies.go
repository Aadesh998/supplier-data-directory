package api

import (
	"net/http"

	"github.com/bharatmfg/directory/internal/httpx"
	"github.com/bharatmfg/directory/internal/notify"
	"github.com/bharatmfg/directory/internal/store"
)

func (s *Server) stats(w http.ResponseWriter, r *http.Request) error {
	st, err := store.GetStats(r.Context(), s.Store.Pool)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, st)
	return nil
}

func (s *Server) adminListCompanies(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	p := httpx.ParsePagination(r, 25)
	data, total, err := store.ListAdminCompanies(r.Context(), s.Store.Pool, store.CompanyFilter{Q: q.Get("q"),
		PublicationStatus: q.Get("publication_status"), VerificationStatus: q.Get("verification_status"),
		Category: q.Get("category"), State: q.Get("state")}, p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

func (s *Server) respondCompany(w http.ResponseWriter, r *http.Request, status int, id string) error {
	c, err := s.Store.GetAdminCompany(r.Context(), s.Store.Pool, id)
	if err != nil {
		return err
	}
	httpx.JSON(w, status, c)
	return nil
}

func (s *Server) adminGetCompany(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	return s.respondCompany(w, r, 200, id)
}

func (s *Server) adminCreateCompany(w http.ResponseWriter, r *http.Request) error {
	var in store.CompanyInput
	if err := httpx.Decode(w, r, &in, maxJSONBody); err != nil {
		return err
	}
	c, err := s.Store.CreateCompany(r.Context(), s.actor(r), in)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, http.StatusCreated, c)
	return nil
}

func (s *Server) adminUpdateCompany(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in store.CompanyInput
	if err := httpx.Decode(w, r, &in, maxJSONBody); err != nil {
		return err
	}
	c, err := s.Store.UpdateCompany(r.Context(), s.actor(r), id, in)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, c)
	return nil
}

func (s *Server) adminAddFacility(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in store.FacilityInput
	if err := httpx.Decode(w, r, &in, maxJSONBody); err != nil {
		return err
	}
	if err := s.Store.AddFacility(r.Context(), s.actor(r), id, in); err != nil {
		return translate(err)
	}
	return s.respondCompany(w, r, http.StatusCreated, id)
}

func (s *Server) adminRemoveFacility(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	fid, err := pathID(r, "facilityId")
	if err != nil {
		return err
	}
	if err := s.Store.RemoveFacility(r.Context(), s.actor(r), id, fid); err != nil {
		return translate(err)
	}
	return s.respondCompany(w, r, 200, id)
}

func (s *Server) adminAddProduct(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in store.ProductInput
	if err := httpx.Decode(w, r, &in, maxJSONBody); err != nil {
		return err
	}
	if err := s.Store.AddProduct(r.Context(), s.actor(r), id, in); err != nil {
		return translate(err)
	}
	return s.respondCompany(w, r, http.StatusCreated, id)
}

func (s *Server) adminRemoveProduct(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	pid, err := pathID(r, "productId")
	if err != nil {
		return err
	}
	if err := s.Store.RemoveProduct(r.Context(), s.actor(r), id, pid); err != nil {
		return translate(err)
	}
	return s.respondCompany(w, r, 200, id)
}

func (s *Server) adminVerify(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in store.VerifyInput
	if err := httpx.Decode(w, r, &in, 16<<10); err != nil {
		return err
	}
	if err := s.Store.Verify(r.Context(), s.actor(r), id, in, s.Cfg.ReverifyInterval); err != nil {
		return translate(err)
	}
	return s.respondCompany(w, r, 200, id)
}

func (s *Server) adminPublish(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.Store.Publish(r.Context(), s.actor(r), id); err != nil {
		return translate(err)
	}
	return s.respondCompany(w, r, 200, id)
}

type reasonBody struct {
	Reason string `json:"reason" validate:"max=2000"`
}

func (s *Server) adminUnpublish(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in reasonBody
	if r.ContentLength != 0 {
		if err := httpx.Decode(w, r, &in, 16<<10); err != nil {
			return err
		}
	}
	if err := s.Store.Unpublish(r.Context(), s.actor(r), id, in.Reason); err != nil {
		return translate(err)
	}
	return s.respondCompany(w, r, 200, id)
}

func (s *Server) adminArchive(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.Store.Archive(r.Context(), s.actor(r), id); err != nil {
		return translate(err)
	}
	return s.respondCompany(w, r, 200, id)
}

// --- submissions ---

func (s *Server) adminListSubmissions(w http.ResponseWriter, r *http.Request) error {
	p := httpx.ParsePagination(r, 25)
	data, total, err := store.ListSubmissions(r.Context(), s.Store.Pool, r.URL.Query().Get("status"), p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

func (s *Server) adminGetSubmission(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	d, err := s.Store.GetSubmission(r.Context(), s.actor(r), id)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, d)
	return nil
}

type approveBody struct {
	Notes string `json:"notes" validate:"max=4000"`
}

func (s *Server) adminApproveSubmission(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in approveBody
	if r.ContentLength != 0 {
		if err := httpx.Decode(w, r, &in, 16<<10); err != nil {
			return err
		}
	}
	companyID, err := s.Store.ApproveSubmission(r.Context(), s.actor(r), id, in.Notes)
	if err != nil {
		return translate(err)
	}
	sub, err := store.GetSubmissionSummary(r.Context(), s.Store.Pool, id)
	if err != nil {
		return err
	}
	if email, name, err := s.Store.SubmitterEmail(r.Context(), id); err == nil {
		d := s.mailData()
		d["CompanyName"], d["SubmissionID"] = name, id
		d["PublicListing"] = sub.Consents.PublicListingAt != nil
		notify.Async(s.Log, s.Mailer, notify.Message{To: email, Template: "submission_approved", Data: d})
	}
	httpx.JSON(w, 200, map[string]any{"submission": sub, "company_id": companyID})
	return nil
}

type rejectBody struct {
	Reason string `json:"reason" validate:"required,max=4000"`
	Status string `json:"status" validate:"omitempty,oneof=rejected needs_changes spam"`
}

func (s *Server) adminRejectSubmission(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in rejectBody
	if err := httpx.Decode(w, r, &in, 16<<10); err != nil {
		return err
	}
	if in.Status == "" {
		in.Status = "rejected"
	}
	if err := s.Store.RejectSubmission(r.Context(), s.actor(r), id, in.Status, in.Reason); err != nil {
		return translate(err)
	}
	sub, err := store.GetSubmissionSummary(r.Context(), s.Store.Pool, id)
	if err != nil {
		return err
	}
	if in.Status != "spam" {
		if email, name, err := s.Store.SubmitterEmail(r.Context(), id); err == nil {
			d := s.mailData()
			d["CompanyName"], d["SubmissionID"], d["Status"], d["Reason"] = name, id, in.Status, in.Reason
			notify.Async(s.Log, s.Mailer, notify.Message{To: email, Template: "submission_rejected", Data: d})
		}
	}
	httpx.JSON(w, 200, map[string]any{"submission": sub})
	return nil
}
