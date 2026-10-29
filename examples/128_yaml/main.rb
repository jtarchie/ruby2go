# YAML: read a config file's settings, then write a report back out as YAML.
require "yaml"

CONFIG = <<~YAML
  site:
    title: My Blog
    url: https://example.com
    draft: no
  defaults: &defaults
    layout: post
    comments: on
  posts:
    - title: Hello
      <<: *defaults
      tags: [intro, meta]
    - title: Second
      <<: *defaults
      layout: page
      words: 1_250
YAML

# Psych's YAML 1.1 rules: `no` and `on` are booleans, `1_250` an Integer; `<<` merges an anchored mapping, which needs aliases: true.
config = YAML.load(CONFIG, aliases: true) #: Hash[String, untyped]
site = config["site"] #: Hash[String, untyped]
puts "#{site["title"]} (#{site["url"]}) draft=#{site["draft"]}"

posts = config["posts"] #: Array[Hash[String, untyped]]
posts.each do |post|
  tags = post.fetch("tags", []) #: Array[String]
  puts "#{post["title"]}: layout=#{post["layout"]} comments=#{post["comments"]} words=#{post.fetch("words", 0)} tags=#{tags.join(",")}"
end

# The default load refuses classes it was not told about; a Date needs permitted_classes.
begin
  YAML.load("published: 2026-10-26")
rescue Psych::DisallowedClass => e
  puts e.message
end
dated = YAML.load("published: 2026-10-26", permitted_classes: [Date]) #: Hash[String, Date]
puts "published on #{dated.fetch("published").strftime("%B %-d, %Y")}"

# Strings that would read back as another type are quoted when dumped, so a round trip keeps them.
report = { "posts" => posts.size, "layouts" => posts.map { |p| p["layout"] }.uniq, "flag" => "yes", "zip" => "02134", "ratio" => 0.5 }
yaml = report.to_yaml
back = YAML.load(yaml) #: Hash[String, untyped]
puts "round trip equal: #{back == report}"
back.each { |k, v| puts "  #{k}: #{v.inspect}" }
