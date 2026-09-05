package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"noder/internal/contract"
	"noder/internal/db"
	"noder/internal/model"
)

func RegisterNodeRoutes(r chi.Router) {
	r.Route("/api/nodes", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Post("/", CreateNode)
		r.Get("/", ListNodes)
		r.Get("/{id}", GetNode)
		r.Put("/{id}", UpdateNode)
		r.Delete("/{id}", DeleteNode)
	})
}

func CreateNode(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	proto, _ := body["protocol"].(string)
	if proto == "" {
		proto = "vless"
		body["protocol"] = proto
	}
	tag, _ := body["tag"].(string)
	nodeName, _ := body["node_name"].(string)
	if nodeName == "" && tag != "" {
		body["node_name"] = tag
	}
	if proto == "tuic" {
		body["security"] = "tls"
		delete(body, "transport_type")
		delete(body, "path")
		delete(body, "public_key")
		delete(body, "short_id")
		delete(body, "fingerprint")
		delete(body, "flow")
		if _, ok := body["congestion_control"]; !ok {
			body["congestion_control"] = "bbr"
		}
	} else {
		delete(body, "congestion_control")
		if _, ok := body["transport_type"]; !ok {
			body["transport_type"] = "direct"
		}
		if _, ok := body["security"]; !ok {
			body["security"] = "tls"
		}
		if fp, _ := body["fingerprint"].(string); fp == "" {
			body["fingerprint"] = "chrome"
		}
		if _, ok := body["flow"]; !ok {
			body["flow"] = "xtls-rprx-vision"
		}
	}
	if _, ok := body["is_active"]; !ok {
		body["is_active"] = true
	}

	if err := contract.ValidateNodeContract(body, proto, true); err != nil {
		HandleAPIError(w, err)
		return
	}

	nodeData, _ := json.Marshal(body)
	var node model.Node
	_ = json.Unmarshal(nodeData, &node)

	if _, err := db.DB.NewInsert().Model(&node).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, node)
}

func ListNodes(w http.ResponseWriter, r *http.Request) {
	var nodes []*model.Node
	err := db.DB.NewSelect().
		Model(&nodes).
		OrderExpr("sort_order ASC, id ASC").
		Scan(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if nodes == nil {
		nodes = []*model.Node{}
	}
	RespondJSON(w, http.StatusOK, nodes)
}

func GetNode(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid node ID")
		return
	}

	var node model.Node
	err = db.DB.NewSelect().Model(&node).Where("id = ?", id).Scan(r.Context())
	if err != nil {
		RespondError(w, http.StatusNotFound, "Node not found")
		return
	}

	RespondJSON(w, http.StatusOK, node)
}

func UpdateNode(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid node ID")
		return
	}

	var existing model.Node
	if err := db.DB.NewSelect().Model(&existing).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Node not found")
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	existingBytes, _ := json.Marshal(existing)
	var merged map[string]interface{}
	_ = json.Unmarshal(existingBytes, &merged)
	for k, v := range body {
		merged[k] = v
	}

	tag, _ := merged["tag"].(string)
	nodeName, _ := merged["node_name"].(string)
	if nodeName == "" && tag != "" {
		merged["node_name"] = tag
	}

	proto, _ := merged["protocol"].(string)
	if proto == "" {
		proto = existing.Protocol
	}

	if proto == "tuic" {
		merged["security"] = "tls"
		merged["transport_type"] = ""
		merged["path"] = nil
		merged["public_key"] = nil
		merged["short_id"] = nil
		merged["fingerprint"] = nil
		merged["flow"] = nil
	} else {
		merged["congestion_control"] = nil
		if t, ok := merged["transport_type"].(string); !ok || t == "" {
			merged["transport_type"] = "direct"
		}
	}

	if err := contract.ValidateNodeContract(merged, proto, true); err != nil {
		HandleAPIError(w, err)
		return
	}

	mergedBytes, _ := json.Marshal(merged)
	_ = json.Unmarshal(mergedBytes, &existing)
	existing.ID = id

	if _, err := db.DB.NewUpdate().Model(&existing).Where("id = ?", id).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, existing)
}

func DeleteNode(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid node ID")
		return
	}

	res, err := db.DB.NewDelete().Model((*model.Node)(nil)).Where("id = ?", id).Exec(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		RespondError(w, http.StatusNotFound, "Node not found")
		return
	}

	// 级联清理关联
	_, _ = db.DB.NewDelete().Model((*model.UserNodeLink)(nil)).Where("node_id = ?", id).Exec(r.Context())

	RespondJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("Node %d deleted successfully", id),
	})
}
