module Schema
  def self.statements
    ["CREATE TABLE IF NOT EXISTS articles (\n  id INTEGER PRIMARY KEY AUTOINCREMENT,\n  title TEXT,\n  body TEXT,\n  created_at TEXT NOT NULL,\n  updated_at TEXT NOT NULL\n)", "CREATE TABLE IF NOT EXISTS comments (\n  id INTEGER PRIMARY KEY AUTOINCREMENT,\n  article_id INTEGER NOT NULL,\n  commenter TEXT,\n  body TEXT,\n  created_at TEXT NOT NULL,\n  updated_at TEXT NOT NULL\n)", "CREATE INDEX IF NOT EXISTS index_comments_on_article_id ON comments (article_id)"]
  end
end
