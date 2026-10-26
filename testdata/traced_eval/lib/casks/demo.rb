cask "demo" do
  version "1.2"

  postflight do
    step("#{File.basename(__FILE__)}:#{__LINE__}")
  end
end
