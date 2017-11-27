Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/trusty64"
  config.vm.box_download_insecure = true
  if Vagrant.has_plugin?("vagrant-proxyconf")
    config.proxy.http     = "http://iss-americas-pitc-cincinnatiz.proxy.corporate.ge.com:80"
    config.proxy.https    = "http://iss-americas-pitc-cincinnatiz.proxy.corporate.ge.com:80"
    config.proxy.no_proxy = "localhost,.ge.com,.build.ge.com,.sw.ge.com,.swcoe.ge.com,.ice.ge.com,.power.ge.com"
  end
end
