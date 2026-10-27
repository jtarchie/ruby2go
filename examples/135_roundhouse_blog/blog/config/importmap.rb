module Importmap
  def self.pins
    [{ name: "application", path: "/assets/application.js" }, { name: "@hotwired/turbo-rails", path: "/assets/turbo.min.js" }, { name: "@hotwired/stimulus", path: "/assets/stimulus.min.js" }, { name: "@hotwired/stimulus-loading", path: "/assets/stimulus-loading.js" }, { name: "controllers/application", path: "/assets/controllers/application.js" }, { name: "controllers/hello_controller", path: "/assets/controllers/hello_controller.js" }, { name: "controllers", path: "/assets/controllers/index.js" }]
  end

  def self.entry
    "application"
  end
end
