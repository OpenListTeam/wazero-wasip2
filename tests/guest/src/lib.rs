wit_bindgen::generate!({
    path: "wit",
    world: "test-world",
    generate_all,
});

use wasi_dep::sockets::{
    instance_network,
    network::{IpAddress, IpAddressFamily, IpSocketAddress, Ipv4Address, Ipv4SocketAddress},
    tcp, tcp_create_socket, udp, udp_create_socket,
};

use crate::wasi::io::poll;
use crate::wasi::io::streams;
use std::cell::RefCell;

// Use a static variable to store state between calls for testing purposes.
thread_local! {
    static GUEST_STATE: RefCell<String> = RefCell::new(String::new());
}

// Define the main struct that will implement the WIT world.
struct MyGuest;

// Implement the `exports` specified in the WIT file.
impl Guest for MyGuest {
    // 实现 test-read-stream 函数
    fn test_read_stream(s: streams::InputStream) -> String {
        // 1. 订阅输入流以获取一个 pollable
        let pollable = s.subscribe();

        // 2. 使用 poll::poll 来阻塞等待 pollable 就绪
        //    我们只传入一个 pollable，所以返回的索引列表里只会有 0
        let ready_indexes = poll::poll(&[&pollable]);
        assert_eq!(ready_indexes, vec![0]);

        // 3. pollable 就绪后，调用 blocking_read 读取数据
        //    这里使用 blocking-read 确保我们能拿到数据
        let max_bytes_to_read: u64 = 4096;
        let data = s
            .blocking_read(max_bytes_to_read)
            .expect("blocking_read failed");

        // 4. 将读取到的字节转换为字符串并返回
        String::from_utf8(data).expect("invalid utf-8")
    }

    // 实现新的 TCP 测试函数
    fn test_tcp_sockets(port: u16, message: String) -> String {
        let network = instance_network::instance_network();
        let family = IpAddressFamily::Ipv4;

        // 1. 创建 TCP 套接字
        let socket =
            tcp_create_socket::create_tcp_socket(family).expect("failed to create tcp socket");

        // 2. 构造服务器地址
        let remote_address = IpSocketAddress::Ipv4(Ipv4SocketAddress {
            port,
            address: (127, 0, 0, 1),
        });

        // 3. 连接到服务器
        socket
            .start_connect(&network, remote_address)
            .expect("failed to start connect");

        // 4. 轮询直到连接完成
        socket.subscribe().block();
        let (input, output) = socket.finish_connect().expect("failed to finish connect");

        // 5. 发送消息
        output.subscribe().block();
        output
            .write(message.as_bytes())
            .expect("failed to write message");

        // 6. 读取响应
        input.subscribe().block();
        let response = input.read(1024).expect("failed to read response");

        String::from_utf8(response).unwrap()
    }

    // 实现新的 UDP 测试函数
    fn test_udp_sockets(port: u16, message: String) -> String {
        let network = instance_network::instance_network();
        let family = IpAddressFamily::Ipv4;

        // 1. 创建 UDP 套接字
        let socket =
            udp_create_socket::create_udp_socket(family).expect("failed to create udp socket");

        // 2. 绑定到任意本地地址和端口
        let local_addr = IpSocketAddress::Ipv4(Ipv4SocketAddress {
            port: 0, // 任意端口
            address: (0, 0, 0, 0),
        });
        socket
            .start_bind(&network, local_addr)
            .expect("failed to start bind");
        // socket.subscribe().block();
        socket.finish_bind().expect("failed to finish bind");

        // 3. 构造服务器地址
        let remote_address = IpSocketAddress::Ipv4(Ipv4SocketAddress {
            port,
            address: (127, 0, 0, 1),
        });

        // 4. 获取数据报流
        let (incoming, outgoing) = socket.stream(None).expect("failed to get udp streams");

        // 5. 发送数据报
        let datagram = udp::OutgoingDatagram {
            data: message.as_bytes().to_vec(),
            remote_address: Some(remote_address),
        };
        outgoing.send(&[datagram]).expect("failed to send datagram");

        // 6. 等待并接收响应数据报
        incoming.subscribe().block();
        let datagrams = incoming.receive(1).expect("failed to receive datagrams");

        String::from_utf8(datagrams[0].data.clone()).unwrap()
    }
}

// Export the implementation.
export!(MyGuest);
