#!/usr/bin/env ruby

require 'json'
require 'ipaddr'
require 'socket'
require 'webrick'

HOST = ENV.fetch('WAKEY_HELPER_HOST', '127.0.0.1')
PORT = Integer(ENV.fetch('WAKEY_HELPER_PORT', '8788'))
ALLOWED_PATH = '/wake'
HELPER_TOKEN = ENV.fetch('WAKEY_HOST_HELPER_TOKEN', '')

def normalize_mac(value)
  mac = value.to_s.strip.delete(':-').downcase
  raise 'MAC address is required' if mac.empty?
  raise 'Invalid MAC address.' unless mac.match?(/\A[0-9a-f]{12}\z/)
  [mac].pack('H*')
end

def build_magic_packet(mac)
  ("\xFF".b * 6) + (mac.b * 16)
end

def send_magic_packet(mac_address, broadcast_address, port)
  mac = normalize_mac(mac_address)
  packet = build_magic_packet(mac)

  socket = UDPSocket.new(Socket::AF_INET)
  socket.setsockopt(Socket::SOL_SOCKET, Socket::SO_BROADCAST, true)

  begin
    3.times do |attempt|
      socket.send(packet, 0, broadcast_address, port)
      sleep(0.1) if attempt < 2
    end
  ensure
    socket.close
  end
end

server = WEBrick::HTTPServer.new(
  BindAddress: HOST,
  Port: PORT,
  AccessLog: [],
  Logger: WEBrick::Log.new($stdout, WEBrick::Log::INFO)
)

trap('INT') { server.shutdown }
trap('TERM') { server.shutdown }

server.mount_proc '/wake' do |req, res|
  unless req.path == ALLOWED_PATH
    res.status = 404
    res['Content-Type'] = 'application/json'
    res.body = JSON.generate(ok: false, message: 'Not found.')
    next
  end

  unless req.request_method == 'POST'
    res.status = 405
    res['Content-Type'] = 'application/json'
    res.body = JSON.generate(ok: false, message: 'Method not allowed.')
    next
  end

  if !HELPER_TOKEN.empty? && req['X-Wakey-Helper-Token'].to_s != HELPER_TOKEN
    res.status = 401
    res['Content-Type'] = 'application/json'
    res.body = JSON.generate(ok: false, message: 'Unauthorized.')
    next
  end

  begin
    payload = req.body.to_s.empty? ? {} : JSON.parse(req.body)
    mac = payload['macAddress']
    broadcast = payload['broadcastAddress'].to_s.strip
    port = Integer(payload.fetch('port', 9))
    raise 'Invalid broadcast address.' if broadcast.empty?
    raise 'Invalid broadcast address.' unless IPAddr.new(broadcast).ipv4?
    raise 'Invalid UDP port.' unless port.between?(1, 65_535)

    send_magic_packet(mac, broadcast, port)

    res.status = 200
    res['Content-Type'] = 'application/json'
    res.body = JSON.generate(ok: true, message: 'Magic packet sent.')
  rescue => e
    res.status = 400
    res['Content-Type'] = 'application/json'
    res.body = JSON.generate(ok: false, message: e.message)
  end
end

puts "host-helper: listening on #{HOST}:#{PORT}"
server.start
