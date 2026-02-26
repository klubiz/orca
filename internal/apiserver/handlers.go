package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/klubi/orca/internal/store"
	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeJSON serialises data as JSON and writes it to the response.
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

// writeError writes a JSON error envelope to the response.
func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, map[string]string{"error": msg})
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var p v1alpha1.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	p.APIVersion = v1alpha1.APIVersion
	p.Kind = v1alpha1.KindProject
	p.Metadata.UID = uuid.New().String()
	now := time.Now()
	p.Metadata.CreatedAt = now
	p.Metadata.UpdatedAt = now
	p.Status = "Active"

	key := store.ResourceKey(v1alpha1.KindProject, "", p.Metadata.Name)
	if err := s.store.Create(key, &p); err != nil {
		if err == store.ErrAlreadyExists {
			s.writeError(w, http.StatusConflict, "project already exists")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, &p)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	key := store.ResourceKey(v1alpha1.KindProject, "", name)

	var p v1alpha1.Project
	if err := s.store.Get(key, &p); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "project not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, &p)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	prefix := "/" + v1alpha1.KindProject + "/"
	items, err := s.store.List(prefix, func() interface{} { return &v1alpha1.Project{} })
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	projects := make([]*v1alpha1.Project, 0, len(items))
	for _, item := range items {
		projects = append(projects, item.(*v1alpha1.Project))
	}

	s.writeJSON(w, http.StatusOK, projects)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	key := store.ResourceKey(v1alpha1.KindProject, "", name)

	var existing v1alpha1.Project
	if err := s.store.Get(key, &existing); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "project not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var p v1alpha1.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Preserve immutable fields
	p.APIVersion = v1alpha1.APIVersion
	p.Kind = v1alpha1.KindProject
	p.Metadata.Name = name
	p.Metadata.UID = existing.Metadata.UID
	p.Metadata.CreatedAt = existing.Metadata.CreatedAt
	p.Metadata.UpdatedAt = time.Now()

	if err := s.store.Update(key, &p); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, &p)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	key := store.ResourceKey(v1alpha1.KindProject, "", name)

	if err := s.store.Delete(key); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "project not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// AgentPods
// ---------------------------------------------------------------------------

func (s *Server) handleCreateAgentPod(w http.ResponseWriter, r *http.Request) {
	var pod v1alpha1.AgentPod
	if err := json.NewDecoder(r.Body).Decode(&pod); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	project := r.URL.Query().Get("project")
	if project == "" {
		project = pod.Metadata.Project
	}
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project is required (query param or metadata.project)")
		return
	}

	pod.APIVersion = v1alpha1.APIVersion
	pod.Kind = v1alpha1.KindAgentPod
	pod.Metadata.Project = project
	pod.Metadata.UID = uuid.New().String()
	now := time.Now()
	pod.Metadata.CreatedAt = now
	pod.Metadata.UpdatedAt = now
	pod.Status.Phase = v1alpha1.PodPending

	key := store.ResourceKey(v1alpha1.KindAgentPod, project, pod.Metadata.Name)
	if err := s.store.Create(key, &pod); err != nil {
		if err == store.ErrAlreadyExists {
			s.writeError(w, http.StatusConflict, "agentpod already exists")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, &pod)
}

func (s *Server) handleGetAgentPod(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	key := store.ResourceKey(v1alpha1.KindAgentPod, project, name)

	var pod v1alpha1.AgentPod
	if err := s.store.Get(key, &pod); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agentpod not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, &pod)
}

func (s *Server) handleListAgentPods(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")

	var prefix string
	if project != "" {
		prefix = "/" + v1alpha1.KindAgentPod + "/" + project + "/"
	} else {
		prefix = "/" + v1alpha1.KindAgentPod + "/"
	}

	items, err := s.store.List(prefix, func() interface{} { return &v1alpha1.AgentPod{} })
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pods := make([]*v1alpha1.AgentPod, 0, len(items))
	for _, item := range items {
		pods = append(pods, item.(*v1alpha1.AgentPod))
	}

	s.writeJSON(w, http.StatusOK, pods)
}

func (s *Server) handleUpdateAgentPod(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	key := store.ResourceKey(v1alpha1.KindAgentPod, project, name)

	var existing v1alpha1.AgentPod
	if err := s.store.Get(key, &existing); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agentpod not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var pod v1alpha1.AgentPod
	if err := json.NewDecoder(r.Body).Decode(&pod); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	pod.APIVersion = v1alpha1.APIVersion
	pod.Kind = v1alpha1.KindAgentPod
	pod.Metadata.Name = name
	pod.Metadata.Project = project
	pod.Metadata.UID = existing.Metadata.UID
	pod.Metadata.CreatedAt = existing.Metadata.CreatedAt
	pod.Metadata.UpdatedAt = time.Now()

	if err := s.store.Update(key, &pod); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, &pod)
}

func (s *Server) handleDeleteAgentPod(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	key := store.ResourceKey(v1alpha1.KindAgentPod, project, name)

	if err := s.store.Delete(key); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agentpod not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// AgentPools
// ---------------------------------------------------------------------------

func (s *Server) handleCreateAgentPool(w http.ResponseWriter, r *http.Request) {
	var pool v1alpha1.AgentPool
	if err := json.NewDecoder(r.Body).Decode(&pool); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	project := r.URL.Query().Get("project")
	if project == "" {
		project = pool.Metadata.Project
	}
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project is required (query param or metadata.project)")
		return
	}

	pool.APIVersion = v1alpha1.APIVersion
	pool.Kind = v1alpha1.KindAgentPool
	pool.Metadata.Project = project
	pool.Metadata.UID = uuid.New().String()
	now := time.Now()
	pool.Metadata.CreatedAt = now
	pool.Metadata.UpdatedAt = now
	pool.Status.Replicas = 0
	pool.Status.ReadyReplicas = 0
	pool.Status.BusyReplicas = 0

	key := store.ResourceKey(v1alpha1.KindAgentPool, project, pool.Metadata.Name)
	if err := s.store.Create(key, &pool); err != nil {
		if err == store.ErrAlreadyExists {
			s.writeError(w, http.StatusConflict, "agentpool already exists")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, &pool)
}

func (s *Server) handleGetAgentPool(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	key := store.ResourceKey(v1alpha1.KindAgentPool, project, name)

	var pool v1alpha1.AgentPool
	if err := s.store.Get(key, &pool); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agentpool not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, &pool)
}

func (s *Server) handleListAgentPools(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")

	var prefix string
	if project != "" {
		prefix = "/" + v1alpha1.KindAgentPool + "/" + project + "/"
	} else {
		prefix = "/" + v1alpha1.KindAgentPool + "/"
	}

	items, err := s.store.List(prefix, func() interface{} { return &v1alpha1.AgentPool{} })
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pools := make([]*v1alpha1.AgentPool, 0, len(items))
	for _, item := range items {
		pools = append(pools, item.(*v1alpha1.AgentPool))
	}

	s.writeJSON(w, http.StatusOK, pools)
}

func (s *Server) handleUpdateAgentPool(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	key := store.ResourceKey(v1alpha1.KindAgentPool, project, name)

	var existing v1alpha1.AgentPool
	if err := s.store.Get(key, &existing); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agentpool not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var pool v1alpha1.AgentPool
	if err := json.NewDecoder(r.Body).Decode(&pool); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	pool.APIVersion = v1alpha1.APIVersion
	pool.Kind = v1alpha1.KindAgentPool
	pool.Metadata.Name = name
	pool.Metadata.Project = project
	pool.Metadata.UID = existing.Metadata.UID
	pool.Metadata.CreatedAt = existing.Metadata.CreatedAt
	pool.Metadata.UpdatedAt = time.Now()

	if err := s.store.Update(key, &pool); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, &pool)
}

func (s *Server) handleDeleteAgentPool(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	key := store.ResourceKey(v1alpha1.KindAgentPool, project, name)

	if err := s.store.Delete(key); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agentpool not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleScaleAgentPool updates only the replicas count of an AgentPool.
func (s *Server) handleScaleAgentPool(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	var body struct {
		Replicas int `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Replicas < 0 {
		s.writeError(w, http.StatusBadRequest, "replicas must be >= 0")
		return
	}

	key := store.ResourceKey(v1alpha1.KindAgentPool, project, name)

	var pool v1alpha1.AgentPool
	if err := s.store.Get(key, &pool); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agentpool not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pool.Spec.Replicas = body.Replicas
	pool.Metadata.UpdatedAt = time.Now()

	if err := s.store.Update(key, &pool); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, &pool)
}

// ---------------------------------------------------------------------------
// DevTasks
// ---------------------------------------------------------------------------

func (s *Server) handleCreateDevTask(w http.ResponseWriter, r *http.Request) {
	var task v1alpha1.DevTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	project := r.URL.Query().Get("project")
	if project == "" {
		project = task.Metadata.Project
	}
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project is required (query param or metadata.project)")
		return
	}

	task.APIVersion = v1alpha1.APIVersion
	task.Kind = v1alpha1.KindDevTask
	task.Metadata.Project = project
	task.Metadata.UID = uuid.New().String()
	now := time.Now()
	task.Metadata.CreatedAt = now
	task.Metadata.UpdatedAt = now
	task.Status.Phase = v1alpha1.TaskPending

	key := store.ResourceKey(v1alpha1.KindDevTask, project, task.Metadata.Name)
	if err := s.store.Create(key, &task); err != nil {
		if err == store.ErrAlreadyExists {
			s.writeError(w, http.StatusConflict, "devtask already exists")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, &task)
}

func (s *Server) handleGetDevTask(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	key := store.ResourceKey(v1alpha1.KindDevTask, project, name)

	var task v1alpha1.DevTask
	if err := s.store.Get(key, &task); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "devtask not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, &task)
}

func (s *Server) handleListDevTasks(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")

	var prefix string
	if project != "" {
		prefix = "/" + v1alpha1.KindDevTask + "/" + project + "/"
	} else {
		prefix = "/" + v1alpha1.KindDevTask + "/"
	}

	items, err := s.store.List(prefix, func() interface{} { return &v1alpha1.DevTask{} })
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	tasks := make([]*v1alpha1.DevTask, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, item.(*v1alpha1.DevTask))
	}

	s.writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleUpdateDevTask(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	key := store.ResourceKey(v1alpha1.KindDevTask, project, name)

	var existing v1alpha1.DevTask
	if err := s.store.Get(key, &existing); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "devtask not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var task v1alpha1.DevTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task.APIVersion = v1alpha1.APIVersion
	task.Kind = v1alpha1.KindDevTask
	task.Metadata.Name = name
	task.Metadata.Project = project
	task.Metadata.UID = existing.Metadata.UID
	task.Metadata.CreatedAt = existing.Metadata.CreatedAt
	task.Metadata.UpdatedAt = time.Now()

	if err := s.store.Update(key, &task); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, &task)
}

func (s *Server) handleDeleteDevTask(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	project := r.URL.Query().Get("project")
	if project == "" {
		s.writeError(w, http.StatusBadRequest, "project query param is required")
		return
	}

	key := store.ResourceKey(v1alpha1.KindDevTask, project, name)

	if err := s.store.Delete(key); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "devtask not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Logs
// ---------------------------------------------------------------------------

// handleGetLogs returns logs for an AgentPod.
// A real implementation would read from a log store; for now we return an
// empty slice since we don't have a log backend yet.
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, []v1alpha1.LogEntry{})
}

// ---------------------------------------------------------------------------
// Apply (generic create-or-update)
// ---------------------------------------------------------------------------

// handleApply accepts a JSON body that includes a "kind" field. It attempts to
// Create the resource first; if it already exists it falls back to Update.
func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	// First, peek at the kind so we know which concrete type to decode into.
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var meta v1alpha1.TypeMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		s.writeError(w, http.StatusBadRequest, "cannot determine resource kind: "+err.Error())
		return
	}

	now := time.Now()

	switch meta.Kind {
	case v1alpha1.KindProject:
		var p v1alpha1.Project
		if err := json.Unmarshal(raw, &p); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		p.APIVersion = v1alpha1.APIVersion
		p.Kind = v1alpha1.KindProject
		key := store.ResourceKey(v1alpha1.KindProject, "", p.Metadata.Name)

		var existing v1alpha1.Project
		if err := s.store.Get(key, &existing); err == store.ErrNotFound {
			// Create
			p.Metadata.UID = uuid.New().String()
			p.Metadata.CreatedAt = now
			p.Metadata.UpdatedAt = now
			if p.Status == "" {
				p.Status = "Active"
			}
			if err := s.store.Create(key, &p); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.writeJSON(w, http.StatusCreated, &p)
		} else if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
		} else {
			// Update
			p.Metadata.UID = existing.Metadata.UID
			p.Metadata.CreatedAt = existing.Metadata.CreatedAt
			p.Metadata.UpdatedAt = now
			if err := s.store.Update(key, &p); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.writeJSON(w, http.StatusOK, &p)
		}

	case v1alpha1.KindAgentPod:
		var pod v1alpha1.AgentPod
		if err := json.Unmarshal(raw, &pod); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		project := pod.Metadata.Project
		if project == "" {
			s.writeError(w, http.StatusBadRequest, "metadata.project is required for AgentPod")
			return
		}

		pod.APIVersion = v1alpha1.APIVersion
		pod.Kind = v1alpha1.KindAgentPod
		key := store.ResourceKey(v1alpha1.KindAgentPod, project, pod.Metadata.Name)

		var existing v1alpha1.AgentPod
		if err := s.store.Get(key, &existing); err == store.ErrNotFound {
			pod.Metadata.UID = uuid.New().String()
			pod.Metadata.CreatedAt = now
			pod.Metadata.UpdatedAt = now
			pod.Status.Phase = v1alpha1.PodPending
			if err := s.store.Create(key, &pod); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.writeJSON(w, http.StatusCreated, &pod)
		} else if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
		} else {
			pod.Metadata.UID = existing.Metadata.UID
			pod.Metadata.CreatedAt = existing.Metadata.CreatedAt
			pod.Metadata.UpdatedAt = now
			if err := s.store.Update(key, &pod); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.writeJSON(w, http.StatusOK, &pod)
		}

	case v1alpha1.KindAgentPool:
		var pool v1alpha1.AgentPool
		if err := json.Unmarshal(raw, &pool); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		project := pool.Metadata.Project
		if project == "" {
			s.writeError(w, http.StatusBadRequest, "metadata.project is required for AgentPool")
			return
		}

		pool.APIVersion = v1alpha1.APIVersion
		pool.Kind = v1alpha1.KindAgentPool
		key := store.ResourceKey(v1alpha1.KindAgentPool, project, pool.Metadata.Name)

		var existing v1alpha1.AgentPool
		if err := s.store.Get(key, &existing); err == store.ErrNotFound {
			pool.Metadata.UID = uuid.New().String()
			pool.Metadata.CreatedAt = now
			pool.Metadata.UpdatedAt = now
			pool.Status.Replicas = 0
			pool.Status.ReadyReplicas = 0
			pool.Status.BusyReplicas = 0
			if err := s.store.Create(key, &pool); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.writeJSON(w, http.StatusCreated, &pool)
		} else if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
		} else {
			pool.Metadata.UID = existing.Metadata.UID
			pool.Metadata.CreatedAt = existing.Metadata.CreatedAt
			pool.Metadata.UpdatedAt = now
			if err := s.store.Update(key, &pool); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.writeJSON(w, http.StatusOK, &pool)
		}

	case v1alpha1.KindDevTask:
		var task v1alpha1.DevTask
		if err := json.Unmarshal(raw, &task); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		project := task.Metadata.Project
		if project == "" {
			s.writeError(w, http.StatusBadRequest, "metadata.project is required for DevTask")
			return
		}

		task.APIVersion = v1alpha1.APIVersion
		task.Kind = v1alpha1.KindDevTask
		key := store.ResourceKey(v1alpha1.KindDevTask, project, task.Metadata.Name)

		var existing v1alpha1.DevTask
		if err := s.store.Get(key, &existing); err == store.ErrNotFound {
			task.Metadata.UID = uuid.New().String()
			task.Metadata.CreatedAt = now
			task.Metadata.UpdatedAt = now
			task.Status.Phase = v1alpha1.TaskPending
			if err := s.store.Create(key, &task); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.writeJSON(w, http.StatusCreated, &task)
		} else if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
		} else {
			task.Metadata.UID = existing.Metadata.UID
			task.Metadata.CreatedAt = existing.Metadata.CreatedAt
			task.Metadata.UpdatedAt = now
			if err := s.store.Update(key, &task); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.writeJSON(w, http.StatusOK, &task)
		}

	default:
		s.writeError(w, http.StatusBadRequest, "unsupported kind: "+meta.Kind)
	}
}
