require_relative "../../runtime/active_record"

class ApplicationRecord < ActiveRecord::Base
  def self.abstract?
    true
  end

  def to_gid_param
    GlobalID.param("ApplicationRecord", @id)
  end
end
