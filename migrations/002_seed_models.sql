-- Seed Data: Alibaba Cloud Bailian (DashScope) Models
-- Run after 001_init.sql

-- ===== Providers =====
INSERT INTO providers (id, display_name, adapter_type, base_url, config) VALUES
    ('bailian_cn', '百炼 DashScope CN', 'dashscope', 'https://dashscope.aliyuncs.com/compatible-mode/v1', 
     '{"region": "cn-hangzhou", "network": "cn_mainland"}'),
    ('bailian_intl', '百炼 DashScope INTL', 'dashscope', 'https://dashscope-intl.aliyuncs.com/compatible-mode/v1',
     '{"region": "ap-southeast-1", "network": "international"}'),
    ('bailian_ga', '百炼 via Global Accelerator', 'dashscope', 'https://{ga_cname}/compatible-mode/v1',
     '{"region": "global", "network": "ga", "note": "需要配置 GA CNAME"}')
ON CONFLICT (id) DO UPDATE SET 
    display_name = EXCLUDED.display_name,
    base_url = EXCLUDED.base_url,
    config = EXCLUDED.config;

-- ===== Text Models (LLM) =====
INSERT INTO models (model_id, display_name, modality, capabilities, input_price, output_price, price_unit, max_context, max_output, supports_stream) VALUES
    -- Qwen 系列
    ('qwen-max', '通义千问 Max', 'text', '["chat", "completion", "function_call"]', 0.0040, 0.0120, 'per_1k_tokens', 32768, 8192, true),
    ('qwen-plus', '通义千问 Plus', 'text', '["chat", "completion", "function_call"]', 0.0008, 0.0020, 'per_1k_tokens', 131072, 8192, true),
    ('qwen-turbo', '通义千问 Turbo', 'text', '["chat", "completion"]', 0.0003, 0.0006, 'per_1k_tokens', 131072, 8192, true),
    ('qwen-long', '通义千问 Long', 'text', '["chat"]', 0.0005, 0.0020, 'per_1k_tokens', 10000000, 8192, true),
    ('qwen3.7-max', 'Qwen3.7 Max', 'text', '["chat", "completion", "function_call", "reasoning"]', 0.0060, 0.0180, 'per_1k_tokens', 131072, 16384, true),
    ('qwen3.7-plus', 'Qwen3.7 Plus', 'text', '["chat", "completion", "function_call", "reasoning"]', 0.0012, 0.0036, 'per_1k_tokens', 131072, 16384, true),
    ('qwen3.7-flash', 'Qwen3.7 Flash', 'text', '["chat", "completion", "reasoning"]', 0.0001, 0.0003, 'per_1k_tokens', 131072, 16384, true),

    -- Qwen Vision
    ('qwen-vl-max', '通义千问 VL Max', 'text', '["chat", "vision"]', 0.0030, 0.0090, 'per_1k_tokens', 32768, 2048, true),
    ('qwen-vl-plus', '通义千问 VL Plus', 'text', '["chat", "vision"]', 0.0008, 0.0020, 'per_1k_tokens', 32768, 2048, true),

    -- GLM 系列 (via Bailian marketplace)
    ('glm-5.1', 'GLM-5.1', 'text', '["chat", "completion"]', 0.0040, 0.0040, 'per_1k_tokens', 128000, 16000, false),
    ('glm-4.6', 'GLM-4.6', 'text', '["chat", "completion"]', 0.0020, 0.0020, 'per_1k_tokens', 128000, 8000, false),
    ('glm-4.5', 'GLM-4.5', 'text', '["chat", "completion", "reasoning"]', 0.0015, 0.0015, 'per_1k_tokens', 128000, 16000, true),
    ('glm-4.5-air', 'GLM-4.5 Air', 'text', '["chat", "completion", "reasoning"]', 0.0005, 0.0005, 'per_1k_tokens', 128000, 16000, true),

    -- DeepSeek 系列 (via Bailian marketplace)
    ('deepseek-v3', 'DeepSeek V3', 'text', '["chat", "completion", "function_call"]', 0.0010, 0.0020, 'per_1k_tokens', 65536, 8192, true),
    ('deepseek-r1', 'DeepSeek R1', 'text', '["chat", "reasoning"]', 0.0020, 0.0080, 'per_1k_tokens', 65536, 8192, true)
ON CONFLICT (model_id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    capabilities = EXCLUDED.capabilities,
    input_price = EXCLUDED.input_price,
    output_price = EXCLUDED.output_price,
    max_context = EXCLUDED.max_context;

-- ===== Image Models =====
INSERT INTO models (model_id, display_name, modality, capabilities, input_price, output_price, price_unit, supports_stream, is_async) VALUES
    ('wan-image', 'Wan Image', 'image', '["text_to_image"]', NULL, 0.0306, 'per_image', false, true),
    ('wan-2.7-image', 'Wan 2.7 Image', 'image', '["text_to_image", "image_to_image"]', NULL, 0.0306, 'per_image', false, true),
    ('wan-2.7-image-pro', 'Wan 2.7 Image Pro', 'image', '["text_to_image", "image_to_image"]', NULL, 0.0750, 'per_image', false, true),
    ('wan-image-edit', 'Wan Image Edit', 'image', '["image_edit"]', NULL, 0.0306, 'per_image', false, true)
ON CONFLICT (model_id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    output_price = EXCLUDED.output_price,
    capabilities = EXCLUDED.capabilities;

-- ===== Video Models =====
INSERT INTO models (model_id, display_name, modality, capabilities, output_price, price_unit, supports_stream, is_async, metadata) VALUES
    ('wan2.7-t2v', 'Wan 2.7 Text-to-Video', 'video', '["text_to_video"]', 0.0860, 'per_second', false, true,
     '{"resolutions": ["720p", "1080p"], "durations": [5, 10]}'),
    ('wan2.7-i2v', 'Wan 2.7 Image-to-Video', 'video', '["image_to_video"]', 0.1019, 'per_second', false, true,
     '{"resolutions": ["720p", "1080p"], "durations": [5, 10]}'),
    ('wan2.6-t2v', 'Wan 2.6 Text-to-Video', 'video', '["text_to_video"]', 0.0860, 'per_second', false, true,
     '{"resolutions": ["720p"], "durations": [5]}'),
    ('wan2.6-i2v', 'Wan 2.6 Image-to-Video', 'video', '["image_to_video"]', 0.1019, 'per_second', false, true,
     '{"resolutions": ["720p", "1080p"], "durations": [5]}')
ON CONFLICT (model_id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    output_price = EXCLUDED.output_price;

-- ===== Audio Models =====
INSERT INTO models (model_id, display_name, modality, capabilities, input_price, output_price, price_unit, supports_stream, is_async) VALUES
    ('paraformer-v2', 'Paraformer V2 (ASR)', 'audio', '["speech_to_text"]', 0.0060, NULL, 'per_second', true, false),
    ('paraformer-realtime', 'Paraformer Realtime', 'audio', '["speech_to_text", "realtime"]', 0.0060, NULL, 'per_second', true, false),
    ('cosyvoice-v2', 'CosyVoice V2 (TTS)', 'audio', '["text_to_speech"]', NULL, 0.0001, 'per_character', true, false),
    ('cosyvoice-v2-stream', 'CosyVoice V2 Stream', 'audio', '["text_to_speech", "realtime"]', NULL, 0.0001, 'per_character', true, false)
ON CONFLICT (model_id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    input_price = EXCLUDED.input_price,
    output_price = EXCLUDED.output_price;

-- ===== Embedding Models =====
INSERT INTO models (model_id, display_name, modality, capabilities, input_price, output_price, price_unit, max_context, supports_stream) VALUES
    ('text-embedding-v3', 'Text Embedding V3', 'embedding', '["embedding"]', 0.0001, NULL, 'per_1k_tokens', 8192, false),
    ('text-embedding-v2', 'Text Embedding V2', 'embedding', '["embedding"]', 0.0001, NULL, 'per_1k_tokens', 2048, false),
    ('multimodal-embedding-v1', 'Multimodal Embedding V1', 'embedding', '["embedding", "multimodal"]', 0.0002, NULL, 'per_1k_tokens', 8192, false)
ON CONFLICT (model_id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    input_price = EXCLUDED.input_price;

-- ===== Model-Provider Bindings =====
-- Text models: bind to both CN and INTL
INSERT INTO model_providers (model_id, provider_id, priority, upstream_model, is_stream, cost_multiplier, timeout_ms)
SELECT m.model_id, p.provider_id,
    CASE WHEN p.provider_id = 'bailian_cn' THEN 1 ELSE 2 END,
    m.model_id,
    m.supports_stream,
    1.00,
    30000
FROM models m
CROSS JOIN (SELECT id AS provider_id FROM providers WHERE id IN ('bailian_cn', 'bailian_intl')) p
WHERE m.modality = 'text'
ON CONFLICT (model_id, provider_id) DO NOTHING;

-- Image/Video models: primarily INTL (international pricing)
INSERT INTO model_providers (model_id, provider_id, priority, upstream_model, is_stream, cost_multiplier, timeout_ms)
SELECT m.model_id, 'bailian_intl', 1, m.model_id, false, 1.00,
    CASE WHEN m.modality = 'video' THEN 600000 ELSE 60000 END
FROM models m
WHERE m.modality IN ('image', 'video')
ON CONFLICT (model_id, provider_id) DO NOTHING;

-- Audio models: CN only (Paraformer/CosyVoice have better CN support)
INSERT INTO model_providers (model_id, provider_id, priority, upstream_model, is_stream, cost_multiplier, timeout_ms)
SELECT m.model_id, 'bailian_cn', 1, m.model_id, m.supports_stream, 1.00, 30000
FROM models m
WHERE m.modality = 'audio'
ON CONFLICT (model_id, provider_id) DO NOTHING;

-- Embedding models: both CN and INTL
INSERT INTO model_providers (model_id, provider_id, priority, upstream_model, is_stream, cost_multiplier, timeout_ms)
SELECT m.model_id, p.provider_id,
    CASE WHEN p.provider_id = 'bailian_cn' THEN 1 ELSE 2 END,
    m.model_id, false, 1.00, 10000
FROM models m
CROSS JOIN (SELECT id AS provider_id FROM providers WHERE id IN ('bailian_cn', 'bailian_intl')) p
WHERE m.modality = 'embedding'
ON CONFLICT (model_id, provider_id) DO NOTHING;
