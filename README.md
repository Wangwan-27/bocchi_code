# Bocchi4 语言特性文档（Bocchi 4.1 / 解释器 v1.14.0）
2026.10.19  ---by Uk10xxx-WaldoBlank27(2024-2026)
> 文件名建议：`Bocchi4-语言特性文档.md`
> 版本：解释器 `v1.14.0` / 语言核心 `Bocchi 4.1`
> 本文基于旧版 Bocchi3.23.0-Bocchi 4.0.3 文档，对照当前源码逐条校对更新，**保留全部原有示例代码**并修正错误语法。
> 标注：✅ 与源码一致 ｜ ⚠️ 旧文档有误、已修正 ｜ 🆕 4.1 新增或旧文档遗漏

---
## 版本特别强调：
```bocchi
//1. 列表合并与相加，需通过内存操作实现
memory.write("arr", [1, 2]);
memory.append("arr", 3); 
puts memory.getString("arr"); // → [1 2 3 4 5]
```


## 目录

1. [快速开始](#1-快速开始)
2. [命令行工具](#2-命令行工具)
3. [变量与类型](#3-变量与类型)
4. [运算符](#4-运算符)
5. [流程控制](#5-流程控制)
6. [函数](#6-函数)
7. [数据结构](#7-数据结构)
8. [对象与面向对象](#8-对象与面向对象)
9. [输入输出](#9-输入输出)
10. [内置 API](#10-内置-api)
11. [模块与包管理](#11-模块与包管理)
12. [错误处理与延迟](#12-错误处理与延迟)
13. [并发](#13-并发)
14. [测试](#14-测试)
15. [性能分析](#15-性能分析)
16. [拆分调试模式](#16-拆分调试模式)
17. [综合示例集](#17-综合示例集)
18. [常见错误](#18-常见错误)
19. [旧版 → 4.1 变更速查](#19-旧版--41-变更速查)

---

## 1. 快速开始

### 1.1 命令速查

```bash
# 查看完整帮助
bocchi -help

# 安装到系统环境
bocchi -env install

# 检查安装状态
bocchi -env          # 4.1 起：无子参数默认检查状态
bocchi -env status   # 兼容写法

# 卸载
bocchi -env uninstall

# 创建示例脚本
bocchi -start
```

### 1.2 第一个脚本

```bocchi
puts "Hello, Bocchi!";
```

执行：

```bash
bocchi hello.bocchi
```

输出：

```
Hello, Bocchi!
执行用时: 12.3ms
```

---

## 2. 命令行工具

### 2.1 脚本执行与 REPL

| 命令 | 说明 |
|---|---|
| `bocchi <file>` | 执行脚本 |
| `bocchi -repl` / `--repl` | 启动 REPL（首行显示 `Bocchi REPL v1.14.0 (Bocchi 4.1)`） |
| `bocchi -help` / `-h` / `--help` | 帮助 |
| `bocchi -version` | 版本号 |
| `bocchi -start` | 生成 `main.bocchi` 示例 |

### 2.2 环境管理（`-env`）

| 命令 | 说明 |
|---|---|
| `bocchi -env` | 🆕 检查环境状态（等价于 `-env status`） |
| `bocchi -env status` | 检查环境状态（兼容） |
| `bocchi -env install` | 安装到系统（创建 PATH / 文件关联 / 桌面与开始菜单快捷方式） |
| `bocchi -env uninstall` | 卸载 |

### 2.3 包管理

| 命令 | 说明 |
|---|---|
| `bocchi -install <pkg>` | 安装包（支持 `.bocchi` 与 🆕 `.tuka`） |
| `bocchi -install` | 🆕 无包名时安装**解释器自身**到系统路径 |
| `bocchi -uninstall <pkg>` | 卸载包 |
| `bocchi -list` | 列出已安装包 |
| `bocchi -package write <pkg>` | 编辑包信息 |
| `bocchi -package make <pkg>` | 生成加密包 |
| `bocchi -package install <pkg>` | 安装加密包 |

### 2.4 打包可执行文件

```bash
bocchi -link <源脚本> <输出文件> [图标.ico]
```

将解释器与脚本合并成单个可执行文件（Windows 下可通过 `rcedit` 设置图标）。

### 2.5 性能分析

```bash
bocchi -prof script.bocchi
bocchi --profile -profile-memory script.bocchi
bocchi -prof -profile-slow script.bocchi
```

### 2.6 拆分调试模式 🆕

```bash
bocchi "?script.bocchi"
bocchi "?C:\projects\bug.bocchi"
```

将脚本按顶层语句拆分，共享全局变量逐块执行；每块出错时报告 `行:列 + 语句类型 + 错误信息`，不中断后续块，最后汇总失败清单。详见 [§16](#16-拆分调试模式)。

---

## 3. 变量与类型

### 3.1 自动类型推断（`set`）

```bocchi
set varName = value;  // 最常用：自动推导类型

set x = 10;            // 整数（内部为 float64 的整值）
set y = 3.14;          // 浮点数
set s = "hello";       // 字符串
set b = true;          // 布尔
set list1 = [1, 2, 3]; // 列表
set dict1 = {name: "Bob", age: 25}; // 字典
```

### 3.2 多变量一次性声明

```bocchi
set a, b, c, d, e = 0, 0, 0, 0, 0;
puts a, b, c, d, e;   // 输出：0 0 0 0 0
```

### 3.3 显式类型声明（`type`）

```bocchi
type int varName = 10;
type float pi = 3.14;
type bool ok = true;
type string name = "Alice";
type list numbers = [1, 2, 3];
type dict user = {name: "Bob", age: 25};
```

### 3.4 类型块（`type { ... }已弃用`）

### 3.5 静态变量（`static`）

```bocchi
// 全局同步，自动同步 go{} 中的变量
static int varName = 10;        // 整数类型
static float pi = 3.14;         // 浮点数类型
static string name = "Alice";   // 字符串类型
static list numbers = [1, 2, 3]; // 列表类型
static dict user = {name: "Bob", age: 25}; // 字典类型
```

`static` 声明的变量存放在全局静态区，**首次初始化后不再重初始化**，可通过 REPL `static` 命令查看。

### 3.6 类型一览

| 类型 | 说明 |
|---|---|
| `int` / `float` | 数字（运行时统一用 `float64` 存储，`int` 会校验整值） |
| `bool` | 布尔 |
| `string` | 字符串 |
| `list` | 列表 |
| `dict` | 字典 |
| `function` | 函数（fn） |
| `time` | 时间（`time.sleep`） |

---

## 4. 运算符

### 4.1 算术运算符

| 运算符 | 含义 |
|---|---|
| `+` | 加（数字相加，字符串拼接） |
| `-` | 减 |
| `*` | 乘 |
| `/` | 除 |
| `^` | 幂（等价于 `pow`） |

示例：

```bocchi
puts 2 + 3 * 4;   // 输出：14
puts 2 ^ 10;      // 输出：1024
```

### 4.2 比较运算符

`==`（等于）、`!=`（不等于）、`<`（小于）、`>`（大于）、`<=`（小于等于）、`>=`（大于等于）

示例：

```bocchi
puts 5 > 3;   // 输出：true
```

### 4.3 逻辑运算符 ⚠️

> 旧文档称"无显式逻辑运算符"，**错误**。4.1 支持 `and` / `or`，短路求值。

| 运算符 | 说明 |
|---|---|
| `and` | 逻辑与（短路） |
| `or` | 逻辑或（短路） |

```bocchi
if (x > 0 and x < 100) { puts "in range"; }
if (name == "" or name == void) { puts "empty"; }
```

> ⚠️ 注意：`and` / `or` 的返回值是 `bool`（不是原值），因为源码内部先 `isTruthy()` 再 `&&` / `||`。

### 4.4 运算符优先级

`^` > `* /` > `+ -` > 比较 > `and` > `or`

---

## 5. 流程控制

### 5.1 条件语句（`if-else`）

```bocchi
if (x > 0) {
    puts "Positive";
} else if (x < 0) {
    puts "Negative";
} else {
    puts "Zero";
}
```

### 5.2 条件循环（`loop`）

```bocchi
set i = 0;
loop (i < 5) {
    puts i;
    i = i + 1;
}
```

### 5.3 遍历循环（`for item in list`）

```bocchi
for item in [1, 2, 3] {
    puts item;
}
```

> 遍历字典时，`item` 为 `"key:value"` 字符串形式。

### 5.4 计数循环（`for..in`）⚠️

语法：`for..in <var> <count> { ... }`，`var` 从 `0` 到 `count-1`。

```bocchi
for..in i 10 {
    puts "循环次数: " + i.string;    // ⚠️ 必须 .string
}
```

> **旧文档修正**：`"循环次数: " + i` 在源码中会 panic（字符串不能直接 + 数字），**必须 `i.string`**，或写成 `puts "循环次数: ", i;`。

### 5.5 跳出循环（`break`）

```bocchi
loop (true) {
    if (条件) { break; }   // 立即跳出循环
}
```

---

## 6. 函数

### 6.1 普通定义（`fn`）

```bocchi
fn add(a, b) {
    return a + b;
}
puts add(2, 3);   // 输出：5
```

### 6.2 默认参数

```bocchi
fn power(base, exp = 2) {
    return base ^ exp;
}

puts power(3);       // 输出：9
puts power(2, 10);   // 输出：1024
```

### 6.3 可变参数（`...`）

```bocchi
fn sum(...numbers) {
    set total = 0;
    for num in numbers {
        total = total + num;
    }
    return total;
}

puts sum(1, 2, 3);          // 输出：6
puts sum(1, 2, 3, 4, 5);    // 输出：15
```

### 6.5 默认参数示例

```bocchi
fn greet(name, greeting = "Hello") {
    return greeting + ", " + name + "!";
}

puts greet("Alice");              // 输出：Hello, Alice!
puts greet("Bob", "Hi");          // 输出：Hi, Bob!
```

> ⚠️ 旧文档写的是 `greet("Bob"+"Hi")`，这在语法上会把 `"Bob"+"Hi"` 当成**第一个参数**（`name` 变为 `"BobHi"`），`greeting` 仍取默认值 → 输出 `Hello, BobHi!`。要覆盖 `greeting` 需用逗号分隔。

### 6.6 Lambda

```bocchi
set add = lambda(a, b) {
    return a + b;
};
puts add(2, 3);   // 输出：5
```

最小 Lambda 示例：

```bocchi
set min = lambda(a, b) {
    if (a < b) {
        return a;
    } else {
        return b;
    }
};
puts min(3, 5);   // 输出：3
```

### 6.7 尾调用优化

```bocchi
fn fib(n) {
    return fib_iter(n, 0, 1);
}
fn fib_iter(n, a, b) {
    if (n == 0) { return a; }
    return fib_iter(n - 1, b, a + b);   // 尾调用，不增加栈
}
```

### 6.8 `rock()` 🆕

```bocchi
rock() {
    // 空体占位函数，Eval 返回 nil
}
```

> 语法上支持，实际无副作用。可视为"占位/无操作"函数。

---

## 7. 数据结构

### 7.1 列表

**字面量**：

```bocchi
[1, "a", true]
[]
[1, 2, 3,]          // 支持尾随逗号
```

**属性 / 方法**：

| 表达式 | 说明 |
|---|---|
| `list.len` | 长度（⚠️ **无括号**，`list.len()` 会解析错误） |
| `list.add(x)` | 追加 x |
| `list.remove(x)` | 删除第一次出现的 x |
| `list.removeAt(i)` | 删除下标 i |
| `list[i]` | 下标访问 |
| `list[a:b]` / `list[a:b:step]` | 切片（支持负数索引） |

```bocchi
set l = [1, 2, 3];
l.add(4);
puts l.len;        // 输出：4
puts l[1];         // 输出：2
puts l[0:2];       // 输出：[1, 2]
```

**旧文档示例（已修正）**：

```bocchi
list.len;                    // 获取长度（无括号）
set lin = [1,2,3].add(3);    // 添加元素
```

### 7.2 字典

```bocchi
set user = {"name": "Alice", age: 25};
puts user["name"];     // 输出：Alice
user["age"] = 26;
```

> 字典字面量的 key 可以用 `"name"` 或裸标识符 `name`（4.1 支持 `{name: ...}` 风格）。

**旧文档示例**：

```bocchi
set aaa = {"key": "value", "name": "Alice"};
puts aaa["key"];    // 输出：value
```

### 7.3 快速列表生成器 🆕

| 函数 | 说明 |
|---|---|
| `range(n)` | `[0, 1, ..., n-1]` |
| `range(a, b)` | `[a, ..., b-1]` |
| `range(a, b, step)` | 指定步长 |
| `repeat(value, count)` | 重复 count 次 |

**旧文档示例**：

```bocchi
putsf range(5);          // [0,1,2,3,4]
putsf range(1, 6);       // [1,2,3,4,5]
putsf range(0, 10, 2);   // [0,2,4,6,8]
putsf repeat("hi", 3);   // ["hi", "hi", "hi"]

// 传统列表字面量（修复后的）
putsf [1, 2, 3];          // 正常
putsf [];                 // 空列表
putsf [1, 2, 3,];         // 支持尾随逗号
```

> ⚠️ 旧文档列出的 `zeros()` / `ones()` / `seq()` **在当前源码中不可用**（未接入解析器或缺少实现分支），请勿使用。

### 7.4 列表推导式 (暂不支持)

### 7.5 内嵌变量字符串 🆕

在 `puts` / `putsf` / `putsr` 的字符串中可使用 `{var}` 插值：

```bocchi
set name = "Bocchi";
puts "Hello, {name}!";    // 输出：Hello, Bocchi!
```

> 仅在输出语句中生效；其他上下文字符串内的 `{}` 会被原样保留（内部以 `\0` 标记）。

---

## 8. 对象与面向对象

### 8.1 对象定义（`obj`）

```bocchi
obj Animal {
    let name = "动物";
    let age = 0;

    fn speak() {
        puts this$name + " 发出声音";
    }
    fn getAge() {
        return this$age;
    }
}
```

### 8.2 继承（`?`）⚠️

> 旧文档里 "extends" 是概念描述，实际语法是 **`obj 子类 ? 父类`**。

```bocchi
// 定义子类，继承父类
obj Dog ? Animal {
    let breed = "未知品种";
    let name = "1";

    // 覆盖父类方法
    fn speak() {
        return "汪汪！我是" + this$name + "，品种是" + this$breed;
    }
    fn wagTail() {
        return this$name + " 摇尾巴";
    }
}

// 创建对象
set myDog = Dog;
puts myDog$name;    // 输出：1（子类覆盖了父类的 name）
puts myDog$breed;   // 输出：未知品种
puts myDog$speak(); // 输出：汪汪！我是1，品种是未知品种
puts myDog$getAge(); // 输出：0（继承自父类）
```

> **旧文档修正**：旧示例写 `set myDog = Dog;` 时 `myDog$name` 注释为"动物（继承自父类）"，**实际是 `"1"`**，因为子类 `let name = "1"` 覆盖了父类。若要保留父类值，应删掉子类的 `let name = ...`。

### 8.3 访问成员与方法

| 写法 | 说明 |
|---|---|
| `obj$member` | 属性访问（`$`） |
| `obj$method(args)` | 方法调用 |
| `this$member` / `this$method(args)` | 对象内部访问自身 |
| `super.method(args)` | 调用父类方法 |

---

## 9. 输入输出

### 9.1 输出（`puts`）

```bocchi
puts "Hello, World!";          // 输出字符串
puts x, "is the value";       // 输出多个表达式（空格分隔）
puts num : 2;                  // 格式化输出（保留 2 位小数）
```

### 9.2 不换行输出（`putsf`）

```bocchi
putsf "Loading: ";
putsf "50%";
```

**旧文档示例**：

```bocchi
// 不换行输出
putsf "Loading: ";
putsf "50%";

// 支持嵌入变量
set name = "Bocchi";
putsf "Hello, ";
putsf name;
puts "!"; // 最后换行
```

### 9.3 重写输出（`putsr`）🆕

```bocchi
putsr "Progress: [===      ] 30%";
time.sleep(0.5);
putsr "Progress: [======   ] 60%";
time.sleep(0.5);
putsr "Progress: [=========] 100%";
puts ""; // 最后换行
```

**进度条示例**：

```bocchi
for..in i 101 {
    putsr "进度: " + i.string + "%";
    time.sleep(0.1);
}
```

### 9.4 输入（`input`）

```bocchi
set name = input("Enter name: "); // 读取用户输入
```

### 9.5 窗口与弹窗

```bocchi
title("Bocchi Script");        // 设置控制台窗口标题
popup("操作完成！");            // 系统弹窗
clear;                          // 清屏（无括号也行）
```

**综合示例（旧文档保留）**：

```bocchi
clear;
title("Hi");

set a = 3.14159265;
puts "原始值:", a;
puts "向下取整:", floor(a);    // 输出 3
puts "向上取整:", ceil(a);     // 输出 4  
puts "四舍五入:", round(a);    // 输出 3
```

### 9.6 退出

```bocchi
exit;         // 退出码 0
exit(1);      // 指定退出码
exit("2");    // 字符串转数字
```

---

## 10. 内置 API

### 10.1 数学函数 ✅

| API | 说明 |
|---|---|
| `sin(x)` / `cos(x)` / `tan(x)` | 三角函数 |
| `sqrt(x)` | 平方根（参数不能为负） |
| `log(x)` / `log10(x)` | 自然对数 / 常用对数 |
| `pow(a, b)` 或 `a ^ b` | 幂 |
| `floor(x)` / `ceil(x)` / `round(x)` | 向下 / 向上 / 四舍五入 |

**旧文档示例**：

```bocchi
puts sin(3.14/2);
puts cos(0);
puts sqrt(25);
puts log(math.E);

// 测试取整功能
set a = 3.14159265;
puts "向下取整:", floor(a);    // 输出 3
puts "向上取整:", ceil(a);     // 输出 4  
puts "四舍五入:", round(a);    // 输出 3

set b = 2.71828;
puts "b 向下取整:", floor(b);  // 输出 2
puts "b 向上取整:", ceil(b);   // 输出 3
puts "b 四舍五入:", round(b);  // 输出 3

set c = 5.6;
puts "c 四舍五入:", round(c);  // 输出 6

// 也可以在表达式中使用
set result = round(3.14159 * 10) / 10;
puts "保留一位小数:", result;  // 输出 3.1
```

### 10.2 随机数

| API | 说明 |
|---|---|
| `rand()` | `[0, 1)` 浮点 |
| `rand(n)` | 整型 `[0, n)` 或浮点 `[0, n)` |
| `rand(a, b)` | 整型 `[a, b]` 或浮点 `[a, b)` |

```bocchi
puts rand();
puts rand(1, 10);
```

### 10.3 类型转换 ⚠️

> 旧文档的 `change(var, type)` 和 `->` **在源码中不存在**。正确写法：

| 写法 | 说明 |
|---|---|
| `expr.int` | 转整数（截断） |
| `expr.float` | 转浮点 |
| `expr.string` | 转字符串 |
| `set x = expr.string` | 转类型 |

```bocchi
set s = "3.14";
set f = s.float;      // 3.14
set n = 3.9.int;      // 3
set t = 123.string;   // "123"

set float_str = str.float;   // 旧文档写法，实际是 "str".float
puts str(123);               // "123"
```

### 10.4 字符串方法

| 表达式 | 说明 |
|---|---|
| `s.len` | 长度（**无括号**） |
| `s.split(sep)` | 按 sep 分割为列表 |
| `s.replace(old, new)` | 全部替换 |

**旧文档示例**：

```bocchi
set str = "Hello World";

// 字符串分割
set parts = str.split(" ");   // ["Hello", "World"]

// 字符串替换
set new_str = str.replace("World", "Bocchi");  // "Hello Bocchi"
```

### 10.5 文件与目录

| API | 说明 |
|---|---|
| `file.read(path)` | 读取全部内容 |
| `file.read_lines(path)` | 按行读取为列表 |
| `file.write(path, content)` | 覆写文件 |
| `file.append(path, content)` | 追加到文件末尾 |
| `file.mkdir(path)` | 创建目录（递归） |
| `file.delete(path)` | 删除文件 |
| `file.encrypt(path, pwd)` | AES-GCM 加密 → `.enc` |
| `file.decrypt(path, pwd)` | 解密 → `.dec` |
| `file.scan(path)` | 扫描二维码（⚠️ 旧文档误写为 `qr.scan`） |
| `dir.delete(path)` | 递归删除目录 |
| `dir.image.show(path)` | 显示图片（⚠️ 旧文档误写为 `image.show`） |
| `dir.image.addText(path, text, size, x, y, color)` | 在图片上叠加文字；`color` 为 `"R,G,B"` 字符串 |

**旧文档示例**：

```bocchi
set content = file.read("data.txt");
file.write("output.txt", "Hello");
file.mkdir("mydir");
file.delete("temp.txt");
dir.delete("old_dir");

// 加密与解密
file.encrypt("secret.txt", "pass123");
file.decrypt("secret.txt.enc", "pass123");
```

### 10.6 时间

| API | 说明 |
|---|---|
| `time.now()` | 当前时间 |
| `time.sleep(seconds)` | 休眠秒数（支持小数） |

```bocchi
set now = time.now();
puts time.format(now, "2006-01-02 15:04:05");
time.sleep(2);
```

### 10.7 并发（详见 [§13](#13-并发)）

`go { ... }` / `@go { ... }` / `ok(var) = expr;` / `wait_all;`

### 10.8 内存命名空间 🆕

用于 `go{}` 协程间共享数据的线程安全全局内存：

| API | 说明 |
|---|---|
| `memory.write(key, value)` | 写入（深拷贝） |
| `memory.read(key [, default])` | 读取；不存在返回 default 或 nil |
| `memory.has(key)` | 是否存在 |
| `memory.delete(key)` | 删除 |
| `memory.clear()` | 清空 |
| `memory.keys()` | 所有 key 的列表 |
| `memory.size()` | key 数量 |
| `memory.add(key, delta)` | 数字累加 |
| `memory.append(key, value)` | 字符串拼接 / 列表合并 / 数字累加 |
| `memory.getString(key [, default])` / `getStr` | 强制字符串读取 |
| `memory.writeString(key, value)` / `writeStr` | 强制字符串写入 |

```bocchi
memory.write("counter", 0);
go {
    for..in i 1000 {
        memory.add("counter", 1);
    }
}
wait_all;
puts memory.read("counter");   // 1000
```

### 10.9 系统命令与参数

| API | 说明 |
|---|---|
| `os.cmd(str)` | 普通权限执行（危险命令被拦截） |
| `os.admin_cmd(str)` | 请求管理员权限执行（Windows） |
| `os.args` | 参数列表 |
| `os.args[i]` | 第 i 个参数 |
| `os.argc` | 参数数量 |

**旧文档示例**：

```bocchi
set output = os.cmd("dir");         // 普通权限
set admin_output = os.admin_cmd("需要管理员权限的命令");  // 管理员权限

// 命令行参数
set args = os.args;          // 参数列表
set argc = os.argc;          // 参数数量
set first_arg = os.args[0];  // 第一个参数

// 窗口标题
title("Bocchi Script");      // 设置控制台窗口标题
```

**危险命令拦截示例**：

```bocchi
set result = os.cmd("rm -rf /");  // 返回警告信息，不执行
```

### 10.10 键盘 / 鼠标

**键盘**：

| API | 说明 |
|---|---|
| `keyboard.press("a")` | 按下 |
| `keyboard.release("a")` | 释放 |
| `keyboard.type("text")` | 粘贴式输入 |
| `keyboard.record([秒 [, 文件]])` | 🆕 开始键盘记录 |
| `keyboard.stopRecord()` | 🆕 停止记录 |

**旧文档示例**：

```bocchi
// 键盘控制示例
keyboard.press("a");              // 按下 a 键
keyboard.release("a");            // 释放 a 键

// 组合操作 - 模拟快捷键
keyboard.press("ctrl");
keyboard.press("v");
keyboard.release("v");
keyboard.release("ctrl");

keyboard.type("password");        // 以复制的方式输入文本
```

**鼠标**：

| API | 说明 |
|---|---|
| `mouse.move(x, y)` | 移动 |
| `mouse.click()` / `mouse.click(left/right/middle)` | 点击 |
| `mouse.scroll(delta)` | 滚动 |
| `mouse.getPos()` | 返回 `{x, y}` 字典 |

**旧文档示例**：

```bocchi
// 鼠标控制示例
mouse.move(500, 300);
mouse.click();                    // 左键点击
mouse.click(right);               // 右键点击
mouse.scroll(5);                  // 向上滚动

// 获取鼠标位置
set pos = mouse.getPos();
puts "鼠标位置:", pos;
```

### 10.11 图像识别与截图

> ⚠️ **语法修正**：路径在**括号内**，`region` / `confidence` / `time.sleep(...)` 在**括号外**，空格分隔。

| API | 示例 |
|---|---|
| `image_find` | `set r = image_find("btn.png") region(0, 0, 800, 600) confidence=0.8 time.sleep(3);` |
| `image_click` | `image_click("btn.png") left confidence=0.9 time.sleep(3);` |
| `image_wait` | `image_wait("loading.png") confidence=0.7 time.sleep(10);` |
| `image_wait_disappear` | `image_wait_disappear("loading.png") time.sleep(10);` |
| `image_exists` | `if (image_exists("icon.png") confidence=0.6) { puts "存在"; }` |
| `screenshot` | `screenshot("screen.png");` 或 `screenshot("region.png") region(100, 100, 400, 300);` |

**旧文档示例（已修正语法）**：

```bocchi
// 图像查找
set result = image_find("button.png") region(0, 0, 800, 600) confidence=0.8;
if (result["found"]) {
    puts "找到图像，位置: (" + result["x"].string + ", " + result["y"].string + ")";
}

// 图像点击
image_click("button.png") left region(0, 0, 800, 600) confidence=0.9;

// 图像等待
image_wait("loading.png") region(0, 0, 800, 600) confidence=0.7 time.sleep(10);

// 等待图像消失
image_wait_disappear("loading.png") region(0, 0, 800, 600) confidence=0.7 time.sleep(10);

// 检查图像是否存在
if (image_exists("icon.png") confidence=0.6) {
    puts "图像存在";
}

// 屏幕截图
screenshot("screen.png");
screenshot("region.png") region(100, 100, 400, 300);  // 指定区域截图
```

### 10.12 二维码

```bocchi
set data = file.scan("qr_code.png");
```

> ⚠️ 旧文档 `qr.scan(path)` **不存在**，正确是 `file.scan(path)`。

### 10.13 键盘录制 🆕

```bocchi
keyboard.record(10);                          // 记录 10 秒，默认保存到 keyboard_record.txt
keyboard.record(10, "my_record.txt");         // 指定文件
keyboard.record();                             // 持续记录，直到 stopRecord()
keyboard.stopRecord();
```

---

## 11. 模块与包管理

### 11.1 引用模块（`use已弃用（默认自动调用）`）


### 11.2 直接引入脚本（`mod`）🆕

```bocchi
mod "lib/helpers.bocchi";
```

路径限制：必须为相对路径、不含 `..`、扩展名为 `.bocchi` / `.tuka` / 无扩展名。作用：在当前环境中执行目标脚本，等同于"源码包含"。

### 11.3 包安装

```bash
bocchi -install package_name          # 安装包
bocchi -install io3.tuka              # 🆕 支持 .tuka 扩展包
bocchi -env install                   # 安装解释器到系统（安装完会询问是否安装 io3.tuka）
bocchi -list                          # 列出已安装
bocchi -uninstall package_name
```

> 🆕 从 `bocchi -env install` 或 `bocchi -install`（无包名）安装解释器时，会询问是否同时安装当前目录下的 `io3.tuka`；若 `io3` 已安装则**直接覆盖原文件**。

### 11.4 加密包

```bash
bocchi -package write mylib         # 编辑包信息
bocchi -package make mylib          # 生成 mylib.enc.bocchi
bocchi -package install mylib       # 解密并安装
```

---

## 12. 错误处理与延迟

### 12.1 捕获异常（`try / expect`）

```bocchi
try {
    file.read("nonexistent.txt");   // 可能抛出错误
} expect (err) {
    puts "Error: ", err;            // 输出错误信息
}
```

### 12.2 延迟执行（`defer`）

```bocchi
defer puts "Cleanup";         // level 0，最先执行
defer < puts "Late";          // level 1
defer << puts "Latest";       // level 2，最后执行
```

> `<` 越多层级越高，按 level 从小到大执行；同一层内后进先出。

---

## 13. 并发

### 13.1 启动协程

```bocchi
go {
    puts "Async task";
}
```

### 13.2 带错误追踪的协程（`@go（已弃用）`）

### 13.3 同步变量到全局（`ok`）🆕

```bocchi
set localVar = 0;
go {
    // 修改 localVar
    ok(localVar) = 42;   // 将 42 同步到全局 localVar
}
```

> 用于子协程将结果写回全局环境（线程安全）。

### 13.4 等待所有协程（`wait_all`）

```bocchi
go { ... }
go { ... }
wait_all;    // 阻塞直到所有 go / @go 完成
```

---

## 14. 测试

### 14.1 定义测试

```bocchi
test "数学运算测试" {
    assert(2 + 2 == 4, "加法测试");
    assert(5 * 5 == 25, "乘法测试");
    assert(10 / 2 == 5, "除法测试");
};

test "字符串测试" {
    set str = "Hello";
    assert(str.len == 5, "长度测试");         // ⚠️ .len 无括号
    assert(str.replace("H", "J") == "Jello", "替换测试");
};
```

> ⚠️ 旧文档 `str.len()` **错误**：`.len` 无括号。

### 14.2 运行测试

```bocchi
runTests;
```

若脚本中定义了 `test`，在脚本执行完毕后会自动运行；也可显式 `runTests;`。

### 14.3 断言

```bocchi
assert(condition, "错误说明");
assert(condition);   // 消息可选
```

失败时 panic（测试模式下），输出中带 ✓ / ✗ 与耗时。

---

## 15. 性能分析

```bash
bocchi -prof script.bocchi
bocchi --profile -profile-memory script.bocchi
bocchi -prof -profile-slow 50 script.bocchi
```

报告内容：

- 总执行时间 / 内存分配 / 函数调用总数 / 慢操作数量
- 函数性能排行（Top 10，按总耗时）
- 热点代码行（Top 5，按执行次数）
- 慢操作警告（超过阈值，默认 100ms）
- 内存使用建议

---

## 16. 拆分调试模式

```bash
bocchi "?script.bocchi"
```

输出示例：

```
🔧 拆分执行模式: script.bocchi

✅ 解析成功，共 12 个语句块
────────────────────────────────────────────────────────────
✓ [1/12] 行 3:1 —— 执行成功 (main.TypeDeclaration)
✓ [2/12] 行 5:1 —— 执行成功 (main.FunctionDef)
❌ [3/12] 行 15:5 —— 执行失败 (main.FunctionCall)
   错误: Undefined variable: foo
...
────────────────────────────────────────────────────────────
执行完成: 成功 10, 失败 2, 总计 12

❌ 失败清单（请修复以下部分）:
  [第3部分] 行 15 列 5 类型 main.FunctionCall —— 错误: Undefined variable: foo
```

特点：

- 所有语句块**共享同一全局变量环境**
- 每块出错精确报告 `行:列 + 语句类型`
- 某块出错**不中断后续块**，最后汇总失败清单
- 与主流程一致：预先加载已安装包、执行 `wait_all` 与 `defer`
- 拆分模式下**跳过 `clear()`**，避免冲掉调试输出

---

## 17. 综合示例集

### 17.1 变量与运算

```bocchi
// 变量与运算
set x = 10;
set y = 3.14;
puts x + y; // 输出：13.14
```

### 17.2 Lambda 最小值

```bocchi
set min = lambda(a, b) {
    if (a < b) {
        return a;
    } else {
        return b;
    }
};
puts min(3, 5); // 输出：3
```

### 17.3 取整 & putsr 进度条综合

```bocchi
// 测试取整功能
clear;
title("Hi");

set a = 3.14159265;
puts "原始值:", a;
puts "向下取整:", floor(a);    // 输出 3
puts "向上取整:", ceil(a);     // 输出 4  
puts "四舍五入:", round(a);    // 输出 3

set b = 2.71828;
puts "b 原始值:", b;
puts "b 向下取整:", floor(b);  // 输出 2
puts "b 向上取整:", ceil(b);   // 输出 3
puts "b 四舍五入:", round(b);  // 输出 3

set c = 5.6;
puts "c 四舍五入:", round(c);  // 输出 6

// 也可以在表达式中使用
set result = round(3.14159 * 10) / 10;
puts "保留一位小数:", result;  // 输出 3.1

// 不换行输出
putsf "Loading: ";
putsf "50%";

// 重写输出（用于进度条等）
putsr "Progress: [===      ] 30%";
time.sleep(0.5);
putsr "Progress: [======   ] 60%";
time.sleep(0.5);
putsr "Progress: [=========] 100%";
puts ""; // 最后换行

// 支持嵌入变量
set name = "Bocchi";
putsf "Hello, ";
putsf name;
puts "!"; // 最后换行

type bool c = a != b;
puts c;

// 使用快速列表生成器语法
putsf range(5);          // [0,1,2,3,4]
putsf range(1, 6);       // [1,2,3,4,5]
putsf range(0, 10, 2);   // [0,2,4,6,8]
putsf repeat("hi", 3); // ["hi", "hi", "hi"]

// 传统列表字面量（修复后的）
putsf [1, 2, 3];          // 正常
putsf [];                 // 空列表
putsf [1, 2, 3,];         // 支持尾随逗号
puts;
```

### 17.4 默认参数 + 可变参数综合

```bocchi
// 默认参数示例
fn greet(name, greeting = "Hello") {
    return greeting + ", " + name + "!";
}

puts greet("Alice");           // 输出: Hello, Alice!
puts greet("Bob", "Hi");       // 输出: Hi, Bob!

fn sum(...numbers) {
    set total = 0;
    for n in numbers {
        total = total + n;
    }
    return total;
}

puts sum(1, 2, 3);        // 输出: 6
puts sum(1, 2, 3, 4, 5);  // 输出: 15

fn calculate(operation, ...values) {
    if (operation == "add") {
        set result = 0;
        for v in values {
            result = result + v;
        }
        return result;
    } else if (operation == "multiply") {
        set result = 1;
        for v in values {
            result = result * v;
        }
        return result;
    }
}

puts calculate("add", 1, 2, 3);        // 输出: 6
puts calculate("multiply", 2, 3, 4);   // 输出: 24
// 注意：calculate(2, 3, 4) 中 "2" 会作为 operation 参与 == 比较，结果为 nil
```

### 17.5 图像识别自动化

```bocchi
set result = image_find("button.png") region(0, 0, 800, 600) confidence=0.8;
if (result["found"]) {
    puts "找到图像，位置: (" + result["x"].string + ", " + result["y"].string + ")";
}

image_click("button.png") left region(0, 0, 800, 600) confidence=0.9;

image_wait("loading.png") confidence=0.7 time.sleep(10);

image_wait_disappear("loading.png") time.sleep(10);

if (image_exists("icon.png") confidence=0.6) {
    puts "图像存在";
}

screenshot("screen.png");
screenshot("region.png") region(100, 100, 400, 300);
```

### 17.6 鼠标键盘自动化

```bocchi
mouse.move(500, 300);
mouse.click();
mouse.click(right);
mouse.scroll(5);

set pos = mouse.getPos();
puts "鼠标位置:", pos;

keyboard.press("a");
keyboard.release("a");

keyboard.press("ctrl");
keyboard.press("v");
keyboard.release("v");
keyboard.release("ctrl");

keyboard.type("password");
```

### 17.7 测试集

```bocchi
test "数学运算测试" {
    assert(2 + 2 == 4, "加法测试");
    assert(5 * 5 == 25, "乘法测试");
    assert(10 / 2 == 5, "除法测试");
};

test "字符串测试" {
    set str = "Hello";
    assert(str.len == 5, "长度测试");
    assert(str.replace("H", "J") == "Jello", "替换测试");
};

// 运行所有测试
runTests;
```

### 17.8 进度条

```bocchi
for..in i 101 {
    putsr "进度: " + i.string + "%";
    time.sleep(0.1);
}
```

### 17.9 内存跨协程共享

```bocchi
memory.write("counter", 0);

go {
    for..in i 1000 {
        memory.add("counter", 1);
    }
}

go {
    for..in i 1000 {
        memory.add("counter", 1);
    }
}

wait_all;
puts memory.read("counter");    // 2000
```

### 17.10 文件读写 + 追加

```bocchi
file.write("test.txt", "");         // 清空
file.append("test.txt", "line1\n");
file.append("test.txt", "line2\n");

set content = file.read("test.txt");
puts content;

set lines = file.read_lines("test.txt");
puts lines.len;                     // 2
```

---

## 18. 常见错误

| 错误信息 | 原因 | 解决 |
|---|---|---|
| `Undefined variable: xxx` | 变量未声明 | 使用 `set` / `type` / `static` 声明 |
| `Type mismatch: expected ...` | 类型不匹配 | 检查类型或使用 `.int` / `.float` / `.string` 转换 |
| `Invalid operands for +` | 字符串 + 数字 | 使用 `.string` 显式转换 |
| `列表索引越界` | 下标超范围 | 检查 `list.len` |
| `文件读取失败` / `File read error` | 路径错误或无权限 | 检查路径、权限 |
| `非法文件路径` | 路径含 `..` 或绝对路径 | 使用相对安全路径 |
| `图像未找到` / `未找到图像` | 图像识别失败 | 降低 `confidence`、缩小 `region`、检查素材质量 |
| `等待图像出现超时` | 超时未出现 | 增大 `time.sleep(N)` 或减小置信度 |
| `内存不足` | 内存占用过多 | 优化算法 / 使用 `memory` 分批 |
| `WaitAll` 阻塞 | `go` 未完成 | 检查子协程是否死循环或等待外部输入 |
| `clear()` 被跳过 | 拆分调试模式下 | 正常行为，避免冲掉输出 |

---

## 19. 旧版 → 4.1 变更速查

### 19.1 语法层面

| 项目 | 旧文档 | 4.1 实际 |
|---|---|---|
| 版本号 | `1.13.4 / Bocchi 4.0.3` | **`1.14.0 / Bocchi 4.1`** |
| 逻辑运算符 | "无" | ✅ **`and` / `or`** |
| 类型转换 | `change(var, type)` / `->` | ✅ **`expr.int` / `expr.float` / `expr.string`** |
| `for..in` 循环体变量 | 示例混用 `"..." + i` | ✅ **必须 `i.string`** |
| 字符串长度 | `str.len()`（带括号） | ✅ **`str.len`（无括号）** |
| 列表生成器 | `range` / `repeat` / `zeros` / `ones` | ✅ **仅 `range` / `repeat` 可用** |
| 对象继承关键字 | "extends"（概念） | ✅ **`obj 子类 ? 父类`** |

### 19.2 API 名称修正

| 旧文档 | 4.1 正确写法 |
|---|---|
| `qr.scan(path)` | ✅ `file.scan(path)` |
| `image.show(path)` | ✅ `dir.image.show(path)` |
| `encrypt(path, pwd)` | ✅ `file.encrypt(path, pwd)` |
| `decrypt(path, pwd)` | ✅ `file.decrypt(path, pwd)` |
| `image_find("x", region(...), confidence=0.8)` | ✅ `image_find("x") region(...) confidence=0.8` |
| `screenshot("x", region(...))` | ✅ `screenshot("x") region(...)` |
| `str.len()` | ✅ `str.len` |
| `greet("Bob"+"Hi")` 想传两个参数 | ✅ `greet("Bob", "Hi")` |

### 19.3 新增特性（旧文档未收录）

- 🆕 **`memory` 命名空间**：跨协程共享内存（write/read/add/append/has/keys/size/getString/writeString 等）
- 🆕 **`ok(var) = expr;`**：子协程写回全局
- 🆕 **`exit()` / `popup()`**：退出与弹窗
- 🆕 **`mod "file.bocchi"`**：直接引入脚本
- 🆕 **`?<file>` 拆分调试模式**
- 🆕 **`bocchi -env` 默认显示状态**
- 🆕 **`bocchi -install` 无包名时安装解释器自身**
- 🆕 **安装时询问是否安装 `io3.tuka` 扩展；已安装则覆盖**
- 🆕 **`.tuka` 包安装支持**

### 19.4 错误处理、并发、测试（旧文档已包含，保持不变）

- `try / expect(err) { ... }` 错误捕获 ✅
- `defer <` / `defer <<` 延迟执行 ✅
- `go { ... }` / `wait_all;` 并发 ✅
- `test "name" { assert(...); };` / `runTests;` 测试 ✅

---

祝您使用 Bocchi 4.1 编程愉快！ 🎉


# io3 扩展方法文档
ver1.1 2026.10.19

> 文件来源：`set void = void;.txt`
> 语言：Bocchi 4.1
> 本文件定义了一套通用工具函数（数学、列表、字符串、文件、判空），供其他 Bocchi 脚本 `use "io3"` 后调用。

---
## 1.1新增:
```bocchi
sort(list, reverse = false)
//对列表 `list` 进行排序。默认按升序排序，`reverse = true` 时按降序排序。

reverse2(list)
//对列表 `list` 进行反转。
```
## 目录

- [全局常量](#全局常量)
- [数学类](#数学类)
- [列表类](#列表类)
- [字符串类](#字符串类)
- [文件类](#文件类)
- [工具类](#工具类)
- [分类速查表](#分类速查表)
- [依赖关系](#依赖关系)
- [使用示例](#使用示例)

---

## 全局常量

| 名称 | 值 | 用途 |
|---|---|---|
| `void` | `"void"` | 表示"空值"的字符串哨兵 |
| `nil` | `"<nil>"` | 另一个空值标记（本文件定义但未使用） |

---

## 数学类

### `min(...nums)`

求所有参数中的最小值。

- **参数**：任意数量数字（可变参数）
- **返回**：最小值；参数为空时返回 `void`
- **示例**：`min(3, 1, 4, 1, 5) → 1`

```bocchi
fn min(...nums) {
    if (nums.len == 0) {
        return void;
    }
    set m = nums[0];
    for v in nums {
        if (v < m) {
            m = v;
        }
    }
    return m;
}
```

---

### `max(...nums)`

求所有参数中的最大值。

- **参数**：任意数量数字（可变参数）
- **返回**：最大值；参数为空时返回 `void`
- **示例**：`max(3, 1, 4, 1, 5) → 5`

```bocchi
fn max(...nums) {
    if (nums.len == 0) {
        return void;
    }
    set m = nums[0];
    for v in nums {
        if (v > m) {
            m = v;
        }
    }
    return m;
}
```

---

### `sum(...nums)`

求和。

- **参数**：任意数量数字（可变参数）
- **返回**：数字，初始为 `0`
- **示例**：`sum(1, 2, 3) → 6`

```bocchi
fn sum(...nums) {
    set total = 0;
    for v in nums {
        total = total + v;
    }
    return total;
}
```

---

### `avg(...nums)`

求平均值。

- **参数**：任意数量数字（可变参数）
- **返回**：平均数；无参数时返回 `void`
- **示例**：`avg(1, 3, 4324, 2342) → 1667.5`

```bocchi
fn avg(...nums) {
    set total = 0;
    set count = 0;
    for v in nums {
        total = total + v;
        count = count + 1;
    }
    if (count == 0) {
        return void;
    }
    return total / count;
}
```

---

### `abs(x)`

求绝对值。

- **参数**：`x` 数字
- **返回**：非负数
- **示例**：`abs(-10) → 10`

```bocchi
fn abs(x) {
    if (x < 0) {
        return 0 - x;
    }
    return x;
}
```

---

### `formax(a, b)`

截取数字字符串前 `b` 位并转为浮点数。

- **参数**：
  - `a`：数字字符串，如 `"3.1415926"`
  - `b`：截取位数
- **返回**：`float`
- **说明**：
  - `a` 按字符拆成数组 `aa`
  - 若 `aa.len < b`，直接 `a.float`
  - 否则用 `for..in i b+2` 把前 `b+2` 个字符写入 `test.txt`，再 `file.read` 转 `float`
  - **注意**：实现依赖全局中转文件 `test.txt`，且实际多取 2 位（依赖 `float` 解析截断）
- **示例**：`formax("3.1415926", 3) → 3.14`

```bocchi
fn formax(a,b) {
    set aa = a.split("");
    if (aa.len < b) {
        set x = a.float;
    } else {
        set i = 0;
        for..in i b+2 {
            file.append("test.txt", aa[i]);
            i = i + 1;
        }
        set lin = file.read("test.txt");
        set x = lin.float;
    }
    return x;
}
```

---

## 列表类

### `addlist(a, b)(旧版，不建议使用)`合并两个列表。
当前拥有更安全的memory操作，建议使用`memory.append`。

- **参数**：`a`、`b` 两个列表
- **返回**：合并后的新列表
- **说明**：**实现绕远路**——通过 `test.txt` 做序列化中转：
  1. `file.write` 写入 `a.string + "\n" + b.string`
  2. `file.read` 读回
  3. 按行 `split("\n")`
  4. 循环中去掉 `[` `]` 和多余空格，再 `split(" ")`
  5. 最后 `file.delete` 删除中转文件
- **注意**：
  - 循环条件 `i < x.len + 1` 与外部变量 `i` 耦合
  - 循环内 `set a = ...` 反复重新赋值，最终返回最后一次处理结果
  - 含空格或特殊字符的元素容易解析错误
- **示例**：`addlist([1, 2], [3, 4]) → [1, 2, 3, 4]`

```bocchi
set addlist = lambda(a,b) {
    file.write("test.txt", a.string +"\n"+ b.string);
    lin = file.read("test.txt");
    x = lin.split("\n");
    file.delete("test.txt");

    loop (i<x.len+1) {
        i = i + 1;
        set a = x.string.replace("]","");
        set a = a.string.replace("[","");
        set a = a.string.replace("   ","");
        set a = a.string.split(" ");
    }

    return a;
};
```

---

### `del(list1, index)`

删除列表指定下标的元素。

- **参数**：
  - `list1`：列表
  - `index`：要删除的下标
- **返回**：新列表
- **依赖**：`addlist`
- **示例**：`del(["a","b","c","d","e"], 2) → ["a","b","d","e"]`

```bocchi
fn del(list1,index) {
    set l1 = list1[0:index];
    index = index + 1;
    set l2 = list1[index:];
    set l1 = addlist(l1,l2);
    return l1;
}
```

---

### `setlist(listc, index, index2)`

把列表中下标 `index` 的元素替换为 `index2`。

- **参数**：
  - `listc`：列表
  - `index`：要替换的下标
  - `index2`：新值
- **返回**：新列表
- **依赖**：`addlist`
- **示例**：`setlist(["a","b","c"], 1, "X") → ["a","X","c"]`

```bocchi
fn setlist(listc,index,index2) {
    set l1 = listc[0:index];
    index = index + 1;
    set l2 = listc[index:];
    set l1 = addlist(l1,index2);
    set l1 = addlist(l1,l2);
    return l1;
}
```

---

### `isinlist(list1, value)`

判断列表是否包含某个值。

- **参数**：`list1` 列表，`value` 待查值
- **返回**：`bool`
- **示例**：`isinlist([1,2,3,4,5], 3) → true`

```bocchi
fn isinlist(list1, value) {
    for v in list1 {
        if (v == value) {
            return true;
        }
    }
    return false;
}
```

---

### `indexof(list1, value)`

查找元素下标。

- **参数**：`list1` 列表，`value` 待查值
- **返回**：首次出现的下标；找不到返回 `-1`
- **示例**：`indexof([1,2,3,4,5], 3) → 2`

```bocchi
fn indexof(list1, value) {
    set i = 0;
    for v in list1 {
        if (v == value) {
            return i;
        }
        i = i + 1;
    }
    return -1;
}
```

---

### `countof(list1, value)`

统计某值在列表中出现的次数。

- **参数**：`list1` 列表，`value` 待查值
- **返回**：数字
- **示例**：`countof([1,2,3,4,5,3], 3) → 2`

```bocchi
fn countof(list1, value) {
    set c = 0;
    for v in list1 {
        if (v == value) {
            c = c + 1;
        }
    }
    return c;
}
```

---

### `join(list1, sep)`

用分隔符把列表拼成字符串。

- **参数**：`list1` 列表，`sep` 分隔符字符串
- **返回**：字符串
- **说明**：首元素前不加分隔符
- **示例**：`join([1,2,3,4,5], ".") → "1.2.3.4.5"`

```bocchi
fn join(list1, sep) {
    set result = "";
    set first = true;
    for v in list1 {
        if (first) {
            result = result + v.string;
            first = false;
        } else {
            result = result + sep + v.string;
        }
    }
    return result;
}
```

---

### `choice(list1)`

随机取列表中的一个元素。

- **参数**：`list1` 列表
- **返回**：任意元素；空列表返回 `void`
- **示例**：`choice([1,2,3,4,5]) → 随机一个`

```bocchi
fn choice(list1) {
    if (list1.len == 0) {
        return void;
    }
    return list1[rand(0, list1.len - 1)];
}
```

---

## 字符串类

### `trim(s)`

去除字符串中的所有空格。

- **参数**：`s` 字符串
- **返回**：去掉空格的字符串
- **说明**：本语言无原生 `trim`，这里用 `replace(" ", "")` 简化实现，**不处理** `\t`、`\n` 和首尾空白
- **示例**：`trim("  123  ") → "123"`

```bocchi
fn trim(s) {
    set x = s.replace(" ", "").string;
    return x;
}
```

---

### `repeatstring(s, n)`

重复字符串 `n` 次。

- **参数**：`s` 字符串，`n` 次数
- **返回**：字符串
- **示例**：`repeatstring("hello", 3) → "hellohellohello"`

```bocchi
fn repeatstring(s, n) {
    set result = "";
    for..in i n {
        result = result + s;
    }
    return result;
}
```

---

### `padleft(s, pad, total)`

左填充字符串到指定长度。

- **参数**：
  - `s` 原字符串
  - `pad` 填充字符
  - `total` 目标总长度
- **返回**：字符串
- **说明**：用 `split("").len` 判断当前长度
- **示例**：`padleft("5", "0", 3) → "005"`

```bocchi
fn padleft(s, pad, total) {
    set result = s;
    loop (result.split("").len < total) {
        result = pad + result;
    }
    return result;
}
```

---

### `padright(s, pad, total)`

右填充字符串到指定长度。

- **参数**：
  - `s` 原字符串
  - `pad` 填充字符
  - `total` 目标总长度
- **返回**：字符串
- **示例**：`padright("5", "0", 3) → "500"`

```bocchi
fn padright(s, pad, total) {
    set result = s;
    loop (result.split("").len < total) {
        result = result + pad;
    }
    return result;
}
```

---

## 文件类

### `fileadd(file1, txt)`

向文件追加文本。

- **参数**：
  - `file1`：文件路径
  - `txt`：要追加的内容
- **返回**：`""`（空字符串）
- **说明**：
  - 先 `file.read` 读入全局 `file2`
  - 拼接 `txt` 后 `file.write` 回写
  - **会覆盖全局变量 `file2`**
- **示例**：`fileadd("use.txt", "1")` — 把 `"1"` 追加到 `use.txt` 末尾

```bocchi
set fileadd = lambda(file1,txt) {
    file2 = file.read(file1);
    file2 = file2 + txt;
    file.write(file1,file2);
    return "";
};
```

---

## 工具类

### `isVoid(x)`

判断变量是否为空值。

- **参数**：`x` 任意
- **返回**：`bool`
- **说明**：与全局常量 `void`（字符串 `"void"`）比较
- **示例**：`isVoid(void) → true`，`isVoid(1) → false`

```bocchi
fn isVoid(x) {
    return x == void;
}
```

---

## 分类速查表

| 类别 | 方法 |
|---|---|
| **数值** | `min` `max` `sum` `avg` `abs` `formax` |
| **列表** | `addlist` `del` `setlist` `isinlist` `indexof` `countof` `join` `choice` |
| **字符串** | `trim` `repeatstring` `padleft` `padright` |
| **文件** | `fileadd` |
| **工具** | `isVoid` |

---

## 依赖关系

```
addlist ──► del
         └─► setlist
formax ──► test.txt（全局中转文件，未清理）
fileadd ─► file2（全局变量，被覆盖）
```

### 潜在问题

- **`addlist` 是核心但实现脆弱**：依赖 `test.txt` 序列化中转，循环条件 `i < x.len + 1` 与外部变量 `i` 耦合，循环内 `set a = ...` 反复重定义，对含空格或特殊字符的列表元素容易出错。
- **`del` / `setlist` 的正确性完全取决于 `addlist`**，若 `addlist` 解析失败，二者也会失败。
- **`formax` 同样依赖 `test.txt`**，且循环 `for..in i b+2` 会写入 `b+2` 个字符，与"前 b 位"的语义有偏差（多取 2 位），实际依赖 `float` 解析截断。
- **`trim` 是简化版**：只删空格，不处理 `\t`、`\n` 和首尾空白。
- **`void` / `nil` 是普通字符串**，`isVoid` 用 `==` 比较字符串。

---

## 使用示例

### 常用调用

```bocchi
// 数学
puts min(3, 1, 4, 1, 5);       // 1
puts max(3, 1, 4, 1, 5);       // 5
puts sum(1, 2, 3);              // 6
puts avg(1, 3, 4324, 2342);    // 1667.5
puts abs(-10);                  // 10
puts formax("3.1415926", 3);   // 3.14

// 列表
puts isinlist([1,2,3,4,5], 3);           // true
puts indexof([1,2,3,4,5], 3);            // 2
puts countof([1,2,3,4,5,3], 3);          // 2
puts join([1,2,3,4,5], ".");             // "1.2.3.4.5"
puts choice([1,2,3,4,5]);                // 随机
puts del(["a","b","c","d","e"], 2);      // ["a","b","d","e"]
puts setlist(["a","b","c"], 1, "X");     // ["a","X","c"]

// 字符串
puts trim("  123  ");                    // "123"
puts repeatstring("hello", 3);           // "hellohellohello"
puts padleft("5", "0", 3);               // "005"
puts padright("5", "0", 3);              // "500"

// 文件
puts fileadd("use.txt", "1");

// 工具
puts isVoid(void);                       // true
puts isVoid(1);                          // false
```
