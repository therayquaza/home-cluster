package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"dinks/internal/dto"
	"dinks/internal/httpx"
	"dinks/internal/middleware"
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
	out := make([]dto.Partner, 0, len(links))
	for _, l := range links {
		member, err := h.repo.GetMember(r.Context(), l.PartnerSubject)
		if err != nil {
			continue
		}
		out = append(out, dto.Partner{
			Subject:     l.PartnerSubject,
			DisplayName: member.DisplayName,
			LinkedAt:    l.CreatedAt.Format(time.RFC3339),
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
	out := make([]dto.PartnerStatus, 0, len(links))
	for _, l := range links {
		member, err := h.repo.GetMember(r.Context(), l.OwnerSubject)
		if err != nil {
			continue
		}
		ps, err := h.repo.Periods(r.Context(), l.OwnerSubject)
		if err != nil {
			continue
		}
		onPeriod, nextPeriod, reminder := cycleStatus(ps)
		out = append(out, dto.PartnerStatus{
			Subject:     l.OwnerSubject,
			DisplayName: member.DisplayName,
			OnPeriod:    onPeriod,
			NextPeriod:  nextPeriod,
			Reminder:    reminder,
		})
	}
	httpx.JSON(w, http.StatusOK, out)
}
