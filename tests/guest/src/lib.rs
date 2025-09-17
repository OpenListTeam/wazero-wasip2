wit_bindgen::generate!({
    path: "wit",
    world: "test-world",
    generate_all,
});

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
}

// Export the implementation.
export!(MyGuest);
