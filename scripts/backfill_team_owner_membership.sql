-- Backfill: equipos creados antes del fix en team_service.Create nunca insertaban
-- una fila en team_users para el owner, por lo que DELETE /teams/:id?user_id=<owner>
-- fallaba con "el usuario no pertenece a este equipo" incluso para el dueño real.
--
-- Idempotente: solo inserta si no existe ya una membresía activa del owner en el equipo.
-- Correr una vez, manualmente, contra la DB de Supabase/Render, después de deployar el fix.

INSERT INTO team_users (team_id, user_id, role_in_team, status, assignment_date, created_at, updated_at)
SELECT t.id, t.owner_id, 'entrenador', 'active', NOW(), NOW(), NOW()
FROM teams t
WHERE t.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM team_users tu
    WHERE tu.team_id = t.id AND tu.user_id = t.owner_id AND tu.deleted_at IS NULL
  );
