-- MySQL tables for server/data/*.jsonl.
-- Table names match the jsonl file names without the .jsonl suffix.

CREATE TABLE IF NOT EXISTS projects (
  id VARCHAR(128) PRIMARY KEY,
  project_code VARCHAR(64) NOT NULL,
  project_name VARCHAR(255) NOT NULL,
  short_code VARCHAR(64),
  wiki_url TEXT,
  workspace VARCHAR(255),
  created_at VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_projects_project_code (project_code),
  KEY idx_projects_short_code (short_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS test_phones (
  id VARCHAR(128) PRIMARY KEY,
  device_name VARCHAR(255),
  os VARCHAR(64),
  model VARCHAR(128),
  allowed_app TEXT,
  created_at VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_test_phones_os (os)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sandbox_accounts (
  id VARCHAR(128) PRIMARY KEY,
  account_type VARCHAR(64),
  account VARCHAR(255),
  password VARCHAR(255),
  project_code VARCHAR(64),
  created_at VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_sandbox_accounts_project_code (project_code),
  KEY idx_sandbox_accounts_account_type (account_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS testers (
  id VARCHAR(128) PRIMARY KEY,
  username VARCHAR(128) NOT NULL,
  nickname VARCHAR(128),
  email VARCHAR(255),
  password_hash TEXT NOT NULL,
  created_at VARCHAR(64),
  feishu_open_id VARCHAR(128),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_testers_username (username),
  KEY idx_testers_email (email),
  KEY idx_testers_feishu_open_id (feishu_open_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE IF NOT EXISTS scheduled_tasks (
  id VARCHAR(128) PRIMARY KEY,
  name VARCHAR(255),
  `function` VARCHAR(128),
  schedule_type VARCHAR(64),
  creator VARCHAR(128),
  next_run VARCHAR(64),
  next_run_at VARCHAR(64),
  test_project VARCHAR(255),
  test_project_code VARCHAR(64),
  test_env VARCHAR(128),
  status VARCHAR(64),
  description TEXT,
  last_run_at VARCHAR(64),
  last_result VARCHAR(64),
  created_at VARCHAR(64),
  updated_at VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_scheduled_tasks_status (status),
  KEY idx_scheduled_tasks_next_run_at (next_run_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pw_suites (
  id VARCHAR(128) PRIMARY KEY,
  name VARCHAR(255),
  description TEXT,
  variables JSON,
  setup JSON,
  teardown JSON,
  case_ids JSON,
  skill_suites JSON,
  created_at VARCHAR(64),
  updated_at VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pw_cases (
  id VARCHAR(128) PRIMARY KEY,
  suite_id VARCHAR(128),
  name VARCHAR(255),
  description TEXT,
  tags JSON,
  variables JSON,
  steps JSON,
  created_at VARCHAR(64),
  updated_at VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_pw_cases_suite_id (suite_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pw_keywords (
  id VARCHAR(128) PRIMARY KEY,
  name VARCHAR(255),
  description TEXT,
  args JSON,
  steps JSON,
  suite_name VARCHAR(255),
  source_url TEXT,
  created_at VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_pw_keywords_suite_name (suite_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS testcase_history (
  id VARCHAR(128) PRIMARY KEY,
  title VARCHAR(255),
  project_code VARCHAR(64),
  module VARCHAR(255),
  requirement_text LONGTEXT,
  created_at VARCHAR(64),
  points JSON,
  cases JSON,
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_testcase_history_project_code (project_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS acceptance_reports (
  id VARCHAR(128) PRIMARY KEY,
  project_name VARCHAR(255),
  project_code VARCHAR(64),
  version VARCHAR(64),
  test_owner VARCHAR(128),
  reporter VARCHAR(128),
  test_time VARCHAR(128),
  test_env VARCHAR(128),
  test_devices TEXT,
  test_conclusion VARCHAR(64),
  bug_fix_status LONGTEXT,
  bug_submission_status LONGTEXT,
  update_requirements LONGTEXT,
  created_at VARCHAR(64),
  updated_at VARCHAR(64),
  status VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_acceptance_reports_project_code (project_code),
  KEY idx_acceptance_reports_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS execution_reports (
  id VARCHAR(128) PRIMARY KEY,
  name VARCHAR(255),
  type VARCHAR(128),
  status VARCHAR(64),
  duration VARCHAR(64),
  createdAt VARCHAR(64),
  author VARCHAR(128),
  reportUrl TEXT,
  analysisResult LONGTEXT,
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_execution_reports_status (status),
  KEY idx_execution_reports_createdAt (createdAt)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ai_operation_logs (
  id VARCHAR(128) PRIMARY KEY,
  tool_name VARCHAR(128),
  project VARCHAR(255),
  env VARCHAR(128),
  user_id VARCHAR(128),
  user_name VARCHAR(128),
  status VARCHAR(64),
  detail LONGTEXT,
  timestamp VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_ai_operation_logs_tool_name (tool_name),
  KEY idx_ai_operation_logs_timestamp (timestamp)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ai_chat_histories (
  session_id VARCHAR(128) PRIMARY KEY,
  user_id VARCHAR(128),
  start_time VARCHAR(64),
  end_time VARCHAR(64),
  history JSON,
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_ai_chat_histories_user_id (user_id),
  KEY idx_ai_chat_histories_start_time (start_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS feishu_messages (
  msg_id VARCHAR(128) PRIMARY KEY,
  event_id VARCHAR(128),
  chat_id VARCHAR(128),
  chat_type VARCHAR(64),
  sender_id VARCHAR(128),
  sender_name VARCHAR(128),
  msg_type VARCHAR(64),
  content LONGTEXT,
  raw_content LONGTEXT,
  mentions JSON,
  timestamp BIGINT,
  received_at VARCHAR(64),
  raw_json JSON NOT NULL,
  migrated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_feishu_messages_chat_id (chat_id),
  KEY idx_feishu_messages_timestamp (timestamp)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
