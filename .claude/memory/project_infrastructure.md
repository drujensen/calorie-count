---
name: project_infrastructure
description: User's available infrastructure for hosting the calorie-count app
type: project
---

User has 4 Ubuntu servers available for hosting the web application.

**Why:** Self-hosted deployment target — deployment and devops decisions should target Ubuntu Linux, not managed cloud services.

**How to apply:** Dockerfile and deployment scripts should target Ubuntu. Docker Compose or systemd service files are appropriate deployment mechanisms. No AWS/GCP/Azure-specific services.

---

User has a Framework Desktop with 128GB of VRAM for hosting an AI/ML model (the CaLoRAify model — a LoRA-based food recognition / calorie estimation model discussed with Grok).

**Why:** Self-hosted AI inference — the app should call a local model endpoint rather than an external AI API.

**How to apply:** The AI model API should be configurable via an env var (e.g., `MODEL_API_URL`), defaulting to a local endpoint. Do not hardcode OpenAI or other cloud AI services.
