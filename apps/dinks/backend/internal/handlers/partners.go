package handlers

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"dinks/internal/dto"
	"dinks/internal/httpx"
	"dinks/internal/middleware"
	"dinks/internal/model"
	"dinks/internal/repository"
)

func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	inv, err := h.repo.CreateInvite(r.Context(), middleware.Subject(r.Context()))
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to create invite", err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dto.Invite{Code: inv.Code, ExpiresAt: inv.ExpiresAt.Format(time.RFC3339)})
}

func (h *Handler) RedeemInvite(w http.ResponseWriter, r *http.Request) {
	var in dto.RedeemInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	_, err := h.repo.RedeemInvite(r.Context(), strings.TrimSpace(in.Code), middleware.Subject(r.Context()))
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Problem(r, w, http.StatusNotFound, "invite code not found or expired", err)
		return
	}
	if err != nil {
		httpx.Problem(r, w, http.StatusBadRequest, err.Error(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListPartners(w http.ResponseWriter, r *http.Request) {
	links, err := h.repo.PartnersOf(r.Context(), middleware.Subject(r.Context()))
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load partners", err)
		return
	}
	subjects := make([]string, 0, len(links))
	for _, l := range links {
		subjects = append(subjects, l.PartnerSubject)
	}
	members, err := h.repo.MembersBySubject(r.Context(), subjects)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load partners", err)
		return
	}
	out := make([]dto.Partner, 0, len(links))
	for _, l := range links {
		member, ok := members[l.PartnerSubject]
		if !ok {
			continue
		}
		out = append(out, dto.Partner{
			Subject:     l.PartnerSubject,
			DisplayName: member.DisplayName,
			LinkedAt:    l.CreatedAt.Format(time.RFC3339),
			Share:       effectiveShare(l),
		})
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) RevokePartner(w http.ResponseWriter, r *http.Request) {
	switch err := h.repo.RevokePartner(r.Context(), middleware.Subject(r.Context()), r.PathValue("subject")); {
	case errors.Is(err, repository.ErrNotFound):
		httpx.Problem(r, w, http.StatusNotFound, "partner not found", err)
	case err != nil:
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to revoke partner", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// partnerStatuses returns the status-only view a partner sees of the subjects
// shared with them: never the underlying records.
func (h *Handler) PartnerStatuses(w http.ResponseWriter, r *http.Request) {
	links, err := h.repo.SharedWithMe(r.Context(), middleware.Subject(r.Context()))
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load shared status", err)
		return
	}
	subjects := make([]string, 0, len(links))
	for _, l := range links {
		subjects = append(subjects, l.OwnerSubject)
	}
	members, err := h.repo.MembersBySubject(r.Context(), subjects)
	if err != nil {
		httpx.Problem(r, w, http.StatusInternalServerError, "unable to load shared status", err)
		return
	}
	// Read each owner's periods concurrently rather than in sequence: with the
	// per-owner read now a single cheap query, serialising them was the dominant
	// cost of this endpoint once several people were shared with.
	type result struct {
		periods []model.Period
		prefs   model.Preferences
		ok      bool
	}
	results := make([]result, len(links))
	var wg sync.WaitGroup
	for i, l := range links {
		if _, ok := members[l.OwnerSubject]; !ok {
			continue
		}
		wg.Add(1)
		go func(i int, owner string) {
			defer wg.Done()
			ps, err := h.repo.StatusPeriods(r.Context(), owner)
			if err != nil {
				return
			}
			results[i] = result{periods: ps, prefs: h.preferencesOf(r.Context(), owner), ok: true}
		}(i, l.OwnerSubject)
	}
	wg.Wait()

	out := make([]dto.PartnerStatus, 0, len(links))
	for i, l := range links {
		member, known := members[l.OwnerSubject]
		if !known || !results[i].ok {
			continue
		}
		// The same projection the detailed view uses, so the summary list can
		// never show a field the owner has since withdrawn.
		out = append(out, h.projectStatus(l, member.DisplayName, results[i].periods, results[i].prefs))
	}
	httpx.JSON(w, http.StatusOK, out)
}
