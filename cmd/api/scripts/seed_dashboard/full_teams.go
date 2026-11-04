package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/crypto"
)

const (
	fullSyntheticSellerAccessToken  = "seed-full-access-token-trainer-1"
	fullSyntheticSellerRefreshToken = "seed-full-refresh-token-trainer-1"
)

type fullTeamSpec struct {
	id, ownerID                                 int64
	key, name, description, level, requirements string
	fee                                         float64
	maxMembers                                  int64
	visible, public                             bool
	street, number                              string
	lat, lng                                    float64
}

type fullGroupSpec struct {
	id, teamID             int64
	key, name, description string
	main                   bool
}

type fullTeamMemberSpec struct {
	teamID, userID           int64
	role, subscriptionStatus string
	initAmount               float64
	paidInstallments         int
	assignmentDate           time.Time
}

type fullGroupMemberSpec struct {
	groupID, userID int64
	start           time.Time
	end             *time.Time
}

func (s *fullSeeder) seedTeams() error {
	ids, err := s.fullTeamUserIDs()
	if err != nil {
		return err
	}
	if config.MyMP.OAuthClientID == "" {
		return fmt.Errorf("MP_OAUTH_CLIENT_ID es obligatorio para seedTeams")
	}
	if config.TokenEncryptionKey == "" {
		return fmt.Errorf("TOKEN_ENCRYPTION_KEY es obligatorio para seedTeams")
	}
	encryptor := crypto.NewAESGCMEncryptor(config.TokenEncryptionKey)
	accessToken, err := encryptor.Encrypt(fullSyntheticSellerAccessToken)
	if err != nil {
		return fmt.Errorf("cifrar access token sintético del seller: %w", err)
	}
	refreshToken, err := encryptor.Encrypt(fullSyntheticSellerRefreshToken)
	if err != nil {
		return fmt.Errorf("cifrar refresh token sintético del seller: %w", err)
	}

	teamSpecs := fullTeamSpecs(s.scope, ids)
	groupSpecs := fullGroupSpecs(s.scope)
	teamMembers := fullTeamMembers(s.scope, ids, teamSpecs, s.now)
	groupMembers := fullGroupMembers(s.scope, ids, s.now)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, spec := range teamSpecs {
			if err := ensureFullTeam(tx, spec, s.now); err != nil {
				return err
			}
		}
		for _, spec := range groupSpecs {
			if err := ensureFullGroup(tx, spec, s.now); err != nil {
				return err
			}
		}
		for _, spec := range teamMembers {
			if err := ensureFullTeamMember(tx, spec, s.now); err != nil {
				return err
			}
		}
		for _, spec := range groupMembers {
			if err := ensureFullGroupMember(tx, spec, s.now); err != nil {
				return err
			}
		}
		for _, spec := range fullInvitationSpecs(s.scope, ids, s.now) {
			if err := ensureFullInvitation(tx, spec, s.now); err != nil {
				return err
			}
		}
		for _, spec := range fullJoinRequestSpecs(s.scope, ids) {
			if err := ensureFullJoinRequest(tx, spec, s.now); err != nil {
				return err
			}
		}
		return ensureFullSellerConnection(tx, ids["trainer_1"], config.MyMP.OAuthClientID, accessToken, refreshToken, s.now)
	})
	if err != nil {
		return fmt.Errorf("sembrar equipos y grupos del perfil full: %w", err)
	}
	for _, spec := range teamSpecs {
		s.ids.teams["team_"+strings.ToLower(spec.key)] = spec.id
	}
	for _, spec := range groupSpecs {
		s.ids.groups[spec.key] = spec.id
	}
	return nil
}

func (s *fullSeeder) fullTeamUserIDs() (map[string]int64, error) {
	keys := []string{
		"trainer_1", "trainer_2", "r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9", "r10", "r11", "r12", "invitations_off",
	}
	ids := make(map[string]int64, len(keys))
	for _, key := range keys {
		id := s.ids.users[key]
		if id <= 0 {
			return nil, fmt.Errorf("seedTeams requiere el ID de identidad full %q; ejecutar seedIdentity antes", key)
		}
		ids[key] = id
	}
	return ids, nil
}

func fullTeamSpecs(scope fullScope, users map[string]int64) []fullTeamSpec {
	return []fullTeamSpec{
		{
			id: scope.teamAID, ownerID: users["trainer_1"], key: "A", name: "Semilla Full · Equipo A Normal",
			description: "[seed_dashboard] [seed_full] Equipo normal del perfil full.", level: "intermedio",
			requirements: "Sin requisitos especiales.", maxMembers: 20, visible: true, public: true,
			street: "Avenida del Parque", number: "100", lat: -31.4201, lng: -64.1888,
		},
		{
			id: scope.teamBID, ownerID: users["trainer_2"], key: "B", name: "Semilla Full · Equipo B Pago",
			description: fullSeedMarker + " Equipo con cuota del perfil full.", level: "avanzado",
			requirements: "Cuota mensual.", fee: 5000, maxMembers: 20, visible: true, public: true,
			street: "Calle del Río", number: "200", lat: -31.4167, lng: -64.1833,
		},
		{
			id: scope.teamCID, ownerID: users["trainer_1"], key: "C", name: "Semilla Full · Equipo C Lleno",
			description: fullSeedMarker + " Equipo con el cupo alcanzado.", level: "principiante",
			requirements: "Cupo completo.", maxMembers: 2, visible: true, public: true,
			street: "Bulevar Central", number: "300", lat: -31.4212, lng: -64.1902,
		},
		{
			id: scope.teamDID, ownerID: users["trainer_2"], key: "D", name: "Semilla Full · Equipo D Oculto",
			description: fullSeedMarker + " Equipo excluido de la búsqueda.", level: "intermedio",
			requirements: "Solo por invitación.", maxMembers: 20, visible: false, public: true,
			street: "Pasaje Norte", number: "400", lat: -31.4144, lng: -64.1811,
		},
		{
			id: scope.teamEID, ownerID: users["trainer_2"], key: "E", name: "Semilla Full · Equipo E Cerrado",
			description: fullSeedMarker + " Equipo que no acepta solicitudes.", level: "avanzado",
			requirements: "Ingreso por invitación.", maxMembers: 20, visible: true, public: false,
			street: "Camino Sur", number: "500", lat: -31.4244, lng: -64.1955,
		},
		{
			id: scope.teamFID, ownerID: users["trainer_1"], key: "F", name: "Semilla Full · Equipo F Colisión",
			description: fullSeedMarker + " Equipo para escenarios de colisión y mensualidad.", level: "intermedio",
			requirements: "Cuota mensual.", fee: 4500, maxMembers: 20, visible: true, public: true,
			street: "Avenida del Parque", number: "600", lat: -31.4201, lng: -64.1888,
		},
	}
}

func fullGroupSpecs(scope fullScope) []fullGroupSpec {
	return []fullGroupSpec{
		{scope.groupAMainID, scope.teamAID, "group_a_main", "Semilla Full · Grupo A principal", "[seed_dashboard] " + fullSeedMarker + " Grupo principal del equipo A.", true},
		{scope.groupASecondaryID, scope.teamAID, "group_a_secondary", "Semilla Full · Grupo A secundario", fullSeedMarker + " Grupo secundario del equipo A.", false},
		{scope.groupBID, scope.teamBID, "group_b", "Semilla Full · Grupo B", fullSeedMarker + " Grupo del equipo B.", true},
		{scope.groupCID, scope.teamCID, "group_c", "Semilla Full · Grupo C", fullSeedMarker + " Grupo del equipo C.", true},
		{scope.groupDID, scope.teamDID, "group_d", "Semilla Full · Grupo D", fullSeedMarker + " Grupo del equipo D.", true},
		{scope.groupEID, scope.teamEID, "group_e", "Semilla Full · Grupo E", fullSeedMarker + " Grupo del equipo E.", true},
		{scope.groupFID, scope.teamFID, "group_f", "Semilla Full · Grupo F", fullSeedMarker + " Grupo del equipo F.", true},
	}
}

func fullTeamMembers(scope fullScope, users map[string]int64, teams []fullTeamSpec, now time.Time) []fullTeamMemberSpec {
	fees := make(map[int64]float64, len(teams))
	for _, spec := range teams {
		fees[spec.id] = spec.fee
	}
	var members []fullTeamMemberSpec
	add := func(teamID int64, userKey, role string) {
		status := ""
		amount := float64(0)
		paid := 0
		assignmentDate := now
		if role == string(constants.TeamUserRoleCorredor) {
			status = string(constants.SubscriptionStatusActive)
			if fees[teamID] > 0 {
				status = string(constants.SubscriptionStatusFirstPaymentPending)
				amount = fees[teamID]
			}
		}
		if teamID == scope.teamFID && userKey == "r10" {
			assignmentDate = fullTeamFR10AssignmentDate(now)
		}
		members = append(members, fullTeamMemberSpec{
			teamID: teamID, userID: users[userKey], role: role,
			subscriptionStatus: status, initAmount: amount, paidInstallments: paid, assignmentDate: assignmentDate,
		})
	}
	add(scope.teamAID, "trainer_1", string(constants.TeamUserRoleEntrenador))
	for _, key := range []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9"} {
		add(scope.teamAID, key, string(constants.TeamUserRoleCorredor))
	}
	add(scope.teamBID, "trainer_2", string(constants.TeamUserRoleEntrenador))
	for _, key := range []string{"r1", "r2", "r6", "r7", "r8", "r11"} {
		add(scope.teamBID, key, string(constants.TeamUserRoleCorredor))
	}
	add(scope.teamCID, "trainer_1", string(constants.TeamUserRoleEntrenador))
	add(scope.teamCID, "r10", string(constants.TeamUserRoleCorredor))
	add(scope.teamDID, "trainer_2", string(constants.TeamUserRoleEntrenador))
	add(scope.teamDID, "r11", string(constants.TeamUserRoleCorredor))
	add(scope.teamEID, "trainer_2", string(constants.TeamUserRoleEntrenador))
	add(scope.teamEID, "r12", string(constants.TeamUserRoleCorredor))
	add(scope.teamFID, "trainer_1", string(constants.TeamUserRoleEntrenador))
	add(scope.teamFID, "r10", string(constants.TeamUserRoleCorredor))
	return members
}

func fullTeamFR10AssignmentDate(now time.Time) time.Time {
	return fullDate(now.AddDate(0, -3, 0)).Add(9 * time.Hour)
}

func fullGroupMembers(scope fullScope, users map[string]int64, now time.Time) []fullGroupMemberSpec {
	var memberships []fullGroupMemberSpec
	add := func(groupID int64, userKey string, start time.Time, end *time.Time) {
		memberships = append(memberships, fullGroupMemberSpec{groupID: groupID, userID: users[userKey], start: start, end: end})
	}
	activeStart := now.AddDate(0, 0, -60)
	for _, key := range []string{"r1", "r2", "r3", "r6", "r7", "r8", "r9"} {
		add(scope.groupAMainID, key, activeStart, nil)
	}
	for _, key := range []string{"r4", "r5"} {
		add(scope.groupASecondaryID, key, activeStart, nil)
	}
	ended := now.AddDate(0, 0, -7)
	add(scope.groupASecondaryID, "r8", activeStart, &ended)
	futureStart := now.AddDate(0, 0, 7)
	add(scope.groupASecondaryID, "r9", futureStart, nil)
	for _, key := range []string{"r1", "r2", "r6", "r7", "r8", "r11"} {
		add(scope.groupBID, key, now.AddDate(0, 0, -30), nil)
	}
	add(scope.groupCID, "r10", now.AddDate(0, 0, -10), nil)
	add(scope.groupDID, "r11", now.AddDate(0, 0, -10), nil)
	add(scope.groupEID, "r12", now.AddDate(0, 0, -10), nil)
	add(scope.groupFID, "r10", now.AddDate(0, 0, -10), nil)
	return memberships
}

func fullInvitationSpecs(scope fullScope, users map[string]int64, now time.Time) []dbs.Invitation {
	return []dbs.Invitation{
		{TeamID: scope.teamAID, InviterID: users["trainer_1"], InviteeID: users["r10"], GroupID: int64Ptr(scope.groupAMainID), Status: string(constants.InvitationStatusPending), ExpiresAt: now.AddDate(0, 0, 14)},
		{TeamID: scope.teamBID, InviterID: users["trainer_2"], InviteeID: users["r9"], GroupID: int64Ptr(scope.groupBID), Status: string(constants.InvitationStatusPending), ExpiresAt: now.AddDate(0, 0, -1)},
		{TeamID: scope.teamBID, InviterID: users["trainer_2"], InviteeID: users["r11"], GroupID: int64Ptr(scope.groupBID), Status: string(constants.InvitationStatusAccepted), ExpiresAt: now.AddDate(0, 0, 14), RespondedAt: timePtr(now)},
		{TeamID: scope.teamAID, InviterID: users["trainer_1"], InviteeID: users["invitations_off"], GroupID: int64Ptr(scope.groupAMainID), Status: string(constants.InvitationStatusRejected), ExpiresAt: now.AddDate(0, 0, 14), RespondedAt: timePtr(now)},
		{TeamID: scope.teamAID, InviterID: users["trainer_1"], InviteeID: users["r12"], GroupID: int64Ptr(scope.groupAMainID), Status: string(constants.InvitationStatusPending), ExpiresAt: now.AddDate(0, 0, -3)},
	}
}

func fullJoinRequestSpecs(scope fullScope, users map[string]int64) []dbs.JoinRequest {
	return []dbs.JoinRequest{
		{TeamID: scope.teamAID, RunnerID: users["r12"], Status: string(constants.InvitationStatusPending)},
		{TeamID: scope.teamEID, RunnerID: users["r10"], Status: string(constants.InvitationStatusPending)},
		{TeamID: scope.teamCID, RunnerID: users["r11"], Status: string(constants.InvitationStatusPending)},
		{TeamID: scope.teamBID, RunnerID: users["r12"], Status: string(constants.InvitationStatusRejected)},
	}
}

func ensureFullTeam(tx *gorm.DB, spec fullTeamSpec, now time.Time) error {
	var existing dbs.Team
	err := tx.Where("id = ?", spec.id).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		team := fullTeamFromSpec(spec, now)
		if err := tx.Select("*").Create(&team).Error; err != nil {
			return fmt.Errorf("crear equipo full %s: %w", spec.key, err)
		}
		return tx.Model(&dbs.Team{}).Where("id = ?", spec.id).Updates(map[string]any{
			"visible": spec.visible, "is_public": spec.public,
		}).Error
	}
	if err != nil {
		return fmt.Errorf("buscar equipo full %s: %w", spec.key, err)
	}
	if !containsFullSeedMarker(existing.Description) {
		return fmt.Errorf("equipo full %s reservado %d existe sin marker %s", spec.key, spec.id, fullSeedMarker)
	}
	team := fullTeamFromSpec(spec, now)
	return tx.Model(&dbs.Team{}).Where("id = ?", spec.id).Updates(map[string]any{
		"name": team.Name, "description": team.Description, "level": team.Level,
		"max_members": team.MaxMembers, "requirements": team.Requirements, "owner_id": team.OwnerID,
		"membership_fee": team.MembershipFee, "status": team.Status,
		"show_groups_to_runners": team.ShowGroupsToRunners, "visible": team.Visible,
		"is_public": team.IsPublic, "country": team.Country, "province": team.Province,
		"city": team.City, "street": team.Street, "number": team.Number,
		"lat": team.Lat, "lng": team.Lng, "deleted_at": nil, "updated_at": now,
	}).Error
}

func fullTeamFromSpec(spec fullTeamSpec, now time.Time) dbs.Team {
	lat, lng := spec.lat, spec.lng
	return dbs.Team{
		ID: spec.id, Name: spec.name, Description: spec.description, Level: spec.level,
		MaxMembers: spec.maxMembers, Requirements: spec.requirements, OwnerID: spec.ownerID,
		MembershipFee: spec.fee, Status: string(constants.TeamStatusActive),
		ShowGroupsToRunners: true, Visible: spec.visible, IsPublic: spec.public,
		Country: "Argentina", Province: "Córdoba", City: "Córdoba", Street: spec.street,
		Number: spec.number, Lat: &lat, Lng: &lng, CreatedAt: now, UpdatedAt: now,
	}
}

func ensureFullGroup(tx *gorm.DB, spec fullGroupSpec, now time.Time) error {
	var existing dbs.Group
	err := tx.Where("id = ?", spec.id).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		group := dbs.Group{
			ID: spec.id, TeamID: spec.teamID, Name: spec.name, Description: spec.description,
			IsMain: spec.main, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&group).Error; err != nil {
			return fmt.Errorf("crear grupo full %d: %w", spec.id, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("buscar grupo full %d: %w", spec.id, err)
	}
	if !containsFullSeedMarker(existing.Description) {
		return fmt.Errorf("grupo full reservado %d existe sin marker %s", spec.id, fullSeedMarker)
	}
	return tx.Model(&dbs.Group{}).Where("id = ?", spec.id).Updates(map[string]any{
		"team_id": spec.teamID, "name": spec.name, "description": spec.description,
		"is_main": spec.main, "deleted_at": nil, "updated_at": now,
	}).Error
}

func ensureFullTeamMember(tx *gorm.DB, spec fullTeamMemberSpec, now time.Time) error {
	var existing dbs.TeamUser
	err := tx.Where("team_id = ? AND user_id = ?", spec.teamID, spec.userID).Take(&existing).Error
	values := map[string]any{
		"role_in_team": spec.role, "status": "active", "subscription_status": nullableString(spec.subscriptionStatus),
		"init_amount": spec.initAmount, "paid_installments": spec.paidInstallments,
		"assignment_date": spec.assignmentDate, "deleted_at": nil, "updated_at": now,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		member := dbs.TeamUser{
			TeamID: spec.teamID, UserID: spec.userID, RoleInTeam: spec.role, Status: "active",
			SubscriptionStatus: spec.subscriptionStatus, InitAmount: spec.initAmount,
			PaidInstallments: spec.paidInstallments, AssignmentDate: spec.assignmentDate, CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&member).Error
	}
	if err != nil {
		return fmt.Errorf("buscar team_user de equipo %d usuario %d: %w", spec.teamID, spec.userID, err)
	}
	return tx.Model(&dbs.TeamUser{}).Where("id = ?", existing.ID).Updates(values).Error
}

func ensureFullGroupMember(tx *gorm.DB, spec fullGroupMemberSpec, now time.Time) error {
	var existing dbs.GroupUser
	err := tx.Where("group_id = ? AND user_id = ?", spec.groupID, spec.userID).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		member := dbs.GroupUser{
			GroupID: spec.groupID, UserID: spec.userID, DateStart: spec.start,
			DateEnd: spec.end, CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&member).Error
	}
	if err != nil {
		return fmt.Errorf("buscar group_user de grupo %d usuario %d: %w", spec.groupID, spec.userID, err)
	}
	return tx.Model(&dbs.GroupUser{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"date_start": spec.start, "date_end": spec.end, "deleted_at": nil, "updated_at": now,
	}).Error
}

func ensureFullInvitation(tx *gorm.DB, spec dbs.Invitation, now time.Time) error {
	var existing dbs.Invitation
	err := tx.Where("team_id = ? AND invitee_id = ?", spec.TeamID, spec.InviteeID).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		spec.CreatedAt, spec.UpdatedAt = now, now
		return tx.Create(&spec).Error
	}
	if err != nil {
		return fmt.Errorf("buscar invitación full para equipo %d usuario %d: %w", spec.TeamID, spec.InviteeID, err)
	}
	return tx.Model(&dbs.Invitation{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"inviter_id": spec.InviterID, "group_id": spec.GroupID, "status": spec.Status,
		"expires_at": spec.ExpiresAt, "responded_at": spec.RespondedAt,
		"deleted_at": nil, "updated_at": now,
	}).Error
}

func ensureFullJoinRequest(tx *gorm.DB, spec dbs.JoinRequest, now time.Time) error {
	var existing dbs.JoinRequest
	err := tx.Where("team_id = ? AND runner_id = ?", spec.TeamID, spec.RunnerID).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		spec.CreatedAt, spec.UpdatedAt = now, now
		return tx.Create(&spec).Error
	}
	if err != nil {
		return fmt.Errorf("buscar solicitud full para equipo %d usuario %d: %w", spec.TeamID, spec.RunnerID, err)
	}
	return tx.Model(&dbs.JoinRequest{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"status": spec.Status, "updated_at": now,
	}).Error
}

func ensureFullSellerConnection(tx *gorm.DB, userID int64, clientID, accessToken, refreshToken string, now time.Time) error {
	var existing dbs.SellerConnection
	err := tx.Where("user_id = ? AND client_id = ?", userID, clientID).Take(&existing).Error
	encryptor := crypto.NewAESGCMEncryptor(config.TokenEncryptionKey)
	connection := dbs.SellerConnection{
		UserID: userID, ClientID: clientID, MPUserID: "seed-full-mp-user-trainer-1",
		AccessToken: accessToken, RefreshToken: refreshToken, PublicKey: "TEST-seed-full-trainer-1",
		TokenExpiresAt: timePtr(now.AddDate(1, 0, 0)),
		Status:         string(constants.SellerConnectionStatusAuthorized), CreatedAt: now, UpdatedAt: now,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&connection).Error
	}
	if err != nil {
		return fmt.Errorf("buscar conexión seller sintética: %w", err)
	}
	if plaintext, err := encryptor.Decrypt(existing.AccessToken); err == nil && plaintext == fullSyntheticSellerAccessToken {
		connection.AccessToken = existing.AccessToken
	}
	if plaintext, err := encryptor.Decrypt(existing.RefreshToken); err == nil && plaintext == fullSyntheticSellerRefreshToken {
		connection.RefreshToken = existing.RefreshToken
	}
	return tx.Model(&dbs.SellerConnection{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"mp_user_id": connection.MPUserID, "access_token": connection.AccessToken,
		"refresh_token": connection.RefreshToken, "public_key": connection.PublicKey,
		"token_expires_at": connection.TokenExpiresAt, "status": connection.Status,
		"updated_at": now,
	}).Error
}

func containsFullSeedMarker(value string) bool {
	return strings.Contains(value, fullSeedMarker)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
