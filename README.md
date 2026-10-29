
# Bocchi4.2 语言特性文档（Bocchi 4.2 / 解释器 v1.14.1）
2026.10.28  ---by Uk10xxx-WaldoBlank27(2024-2026)

> 文件名：`Bocchi4.2README.markdown`
> 源码：`Bocchi4.2.go`（13168 行）｜ 可执行文件：`Bocchi4.2.exe`
> 本文基于旧版《Bocchi4.1README / README / io3使用说明》三份文档，对照 **Bocchi4.2.go 源码逐条校对并实际运行验证**，保留原有示例并修正错误语法。
> 标注：✅ 与源码/运行结果一致 ｜ ⚠️ 旧文档有误、已修正 ｜ 🆕 4.2 新增

---

## 版本特别说明

- `bocchi -version` 实际输出：`BOCCHI 1.14.1 (Bocchi 4.1)`（源码中该字符串未随 4.2 更新）
- `bocchi -help` 横幅显示：`版本 1.14.1 (Bocchi 4.1.4)`
- REPL 首行显示：`Bocchi REPL v1.14.0 (Bocchi 4.1)`
- 4.2 相对 4.1.4 的**唯一**语言级新增是 `server` 服务器功能（新增 7 个 Token、7 个关键字），详见 [§14 服务器功能](#14-服务器功能)。

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
14. [服务器功能（4.2 新增）](#14-服务器功能42-新增)
15. [测试](#15-测试)
16. [性能分析](#16-性能分析)
17. [拆分调试模式](#17-拆分调试模式)
18. [测试文件参考（bocchi / tuka）](#18-测试文件参考bocchi--tuka)
19. [常见错误与已知问题](#19-常见错误与已知问题)
20. [4.1 → 4.2 变更速查](#20-41--42-变更速查)
21. [完整测试报告（2026-10-28 实测）](#21-完整测试报告2026-10-28-实测)
22. [代码示例逐条测试报告（2026-10-28 实测）](#22-代码示例逐条测试报告2026-10-28-实测)

---

## 1. 快速开始

### 1.1 命令速查

```bash
# 查看完整帮助
bocchi -help

# 安装到系统环境（需管理员权限）
bocchi -env install

# 检查环境状态（🆕 4.1.4 起还会先做版本检查与扩展更新检测）
bocchi -env
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

> ⚠️ 运行任何脚本前，解释器会**自动加载所有已安装的包**（如 `io4`）。io4 在加载时会执行 `file.write("test.txt", "")` 等顶层语句，因此要求**当前目录可写**；目录只读时会报 `File write error: open test.txt: Access is denied.`。

---

## 2. 命令行工具

### 2.1 脚本执行与 REPL

| 命令 | 说明 |
|---|---|
| `bocchi <file>` | 执行脚本（支持 `.bocchi` / `.tuka` / 无扩展名） |
| `bocchi -repl` / `--repl` | 启动 REPL |
| `bocchi -help` / `-h` / `--help` | 帮助 |
| `bocchi -version` | 版本号（输出 `BOCCHI 1.14.1 (Bocchi 4.1)`） |
| `bocchi -start` | 生成 `main.bocchi` 示例 |

### 2.2 环境管理（`-env`）🆕 更新流程

`bocchi -env`（无子参数）现在按顺序执行三步：

1. **版本检查**：找到系统已安装的 bocchi，若低于当前版本则询问 `是否更新为当前版本（覆盖安装）？(Y/n)`，需要管理员权限；
2. **扩展检查**：扫描当前目录下 `io4.tuka` 及以上的扩展包，若发现则询问是否安装（并自动卸载已安装的旧 `io2` / `io3`）；
3. **环境状态**：显示 PATH / 安装目录 / 文件关联状态。

实际输出示例（本机验证）：

```
📌 Bocchi 已是最新版本: 1.14.1

📦 检测到新扩展包: io4.tuka (版本 4)
   是否安装最新扩展？(Y/n): 已跳过扩展安装
检查Bocchi环境状态...
✅ 在PATH中找到: C:\Program Files\Bocchi\bocchi.exe
✅ 安装目录存在: C:\Program Files\Bocchi
✅ 文件关联已设置
```

| 命令 | 说明 |
|---|---|
| `bocchi -env` | 🆕 版本检查 → 扩展检查 → 环境状态 |
| `bocchi -env install` | 安装 + 同样执行上述三步 |
| `bocchi -env status` | 仅显示环境状态 |
| `bocchi -env uninstall` | 卸载 |

### 2.3 包管理

| 命令 | 说明 |
|---|---|
| `bocchi -install <pkg>` | 安装包（支持 `.bocchi` 与 `.tuka`） |
| `bocchi -install` | 无包名时安装**解释器自身**到系统路径 |
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
bocchi -prof -profile-slow 50 script.bocchi
```

### 2.6 拆分调试模式

```bash
bocchi "?script.bocchi"
bocchi "?C:\projects\bug.bocchi"
```

详见 [§17](#17-拆分调试模式)。

---

## 3. 变量与类型

### 3.1 自动类型推断（`set`）

```bocchi
set x = 10;            // 整数（内部为 float64 的整值）
set y = 3.14;          // 浮点数
set s = "hello";       // 字符串
set b = true;          // 布尔
set list1 = [1, 2, 3]; // 列表
set dict1 = {"name": "Bob", "age": 25}; // 字典
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
type bool flag = true;
type string name = "Alice";
type list numbers = [1, 2, 3];
type dict user = {"name": "Bob", "age": 25};
```

### 3.4 类型块（`type { ... }`）

已弃用。

### 3.5 静态变量（`static`）

```bocchi
static int varName = 10;        // 整数类型
static float pi = 3.14;         // 浮点数类型
static string name = "Alice";   // 字符串类型
static list numbers = [1, 2, 3]; // 列表类型
static dict user = {"name": "Bob", "age": 25}; // 字典类型
```

`static` 声明的变量存放在全局静态区（`staticVariables`），**首次初始化后不再重初始化**，可通过 REPL `static` 命令查看。

> ⚠️ **实测：`static` 变量初始化后不可再赋值**。普通赋值 `s = 2;` 会把新值写进环境的影子变量，而读取时始终优先返回静态区里的初值，因此 `s = 2; puts s;` 仍输出初值（赋值被静默忽略）。需要可变全局量请用 `memory` 或普通 `set`。

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
| `%` / `mod` | 取模（`math.Mod`，符号跟随被除数） |

```bocchi
puts 2 + 3 * 4;     // 输出：14
puts 2 ^ 10;        // 输出：1024
puts 10 % 3;        // 1
puts 10 mod 3;      // 1（关键字写法也生效）
puts -10 % 3;       // -1
puts 10.5 % 3;      // 1.5
puts 2 ^ 3 % 3;     // 2（^ 与 % 同级，从左到右）
```

### 4.2 比较运算符

`==`（等于）、`!=`（不等于）、`<`（小于）、`>`（大于）、`<=`（小于等于）、`>=`（大于等于）

```bocchi
puts 5 > 3;   // 输出：true
```

### 4.3 逻辑运算符

| 运算符 | 说明 |
|---|---|
| `and` | 逻辑与（短路） |
| `or` | 逻辑或（短路） |

```bocchi
if (x > 0 and x < 100) { puts "in range"; }
if (name == "" or name == void) { puts "empty"; }
```

> ⚠️ `and` / `or` 的返回值是 `bool`（不是原值），内部先 `isTruthy()` 再 `&&` / `||`。

### 4.4 运算符优先级

`^` / `%` > `* /` > `+ -` > 比较 > `and` > `or`

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

### 5.4 计数循环（`for..in`）

语法：`for..in <var> <count> { ... }`，`var` 从 `0` 到 `count-1`。

```bocchi
for..in i 10 {
    puts "循环次数: " + i.string;    // ⚠️ 必须 .string
}
```

### 5.5 跳出循环（`break`）

```bocchi
loop (true) {
    if (条件) { break; }   // 立即跳出循环
}
```

### 5.6 ⚠️ 块语句后面不要写分号（4.2 实测陷阱）

所有块语句（`if` / `loop` / `for` / `for..in` / `go` / `test` / `obj` / `fn` / `type` 块）的收尾 `}` **后面不能加分号**，否则解析报 `同步到分号`：

```bocchi
// ❌ 错误写法（实测报错：同步到分号）
if (x > 0) {
    puts "pos";
};

// ✅ 正确写法
if (x > 0) {
    puts "pos";
}
```

`go { ... }` 在函数体内同样适用（不加分号即可正常解析）：

```bocchi
fn myfn() {
    go {
        puts "async";
    }
    return "ok";
}
```

---

## 6. 函数

### 6.1 普通定义（`fn`）

```bocchi
fn add2(a, b) {
    return a + b;
}
puts add2(2, 3);   // 输出：5
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

### 6.4 匿名函数表达式（`fn(params) { ... }`）🆕 常用写法

4.2 服务器 handler 常用此语法：

```bocchi
server.get("/", fn(req) {
    return "hello";
});
```

### 6.5 Lambda

```bocchi
set add2 = lambda(a, b) {
    return a + b;
};
puts add2(2, 3);   // 输出：5
```

### 6.6 尾调用优化

```bocchi
fn fib(n) {
    return (fib_iter(n, 0, 1));
}
fn fib_iter(n, a, b) {
    if (n == 0) { return a; }
    return (fib_iter(n - 1, b, a + b));
}
puts fib(10);   // 输出：55
```

> ⚠️ 实测：`return fn(...);`（返回**命名函数调用**，不带括号）会解析失败（尾调用快速路径与 parsePrimary 重复解析冲突，报"同步到…"）。加一层括号 `return (fn(...));` 即可正常解析，实测 `fib(10) → 55`。

### 6.7 `rock()`

```bocchi
rock() {
    // 空体占位函数，Eval 返回 nil
}
```

### 6.8 🆕 函数调用记忆化缓存（实测发现）

解释器内置**全局记忆化缓存**（`globalMemoCache`）：对**命名函数**的调用按 `(函数名, 参数串化哈希)` 缓存结果，第二次同参调用直接返回缓存值，不再执行函数体。

**优点**：

- 递归函数指数级加速。实测 `fib_rec(1000)`（朴素递归，`fib数列.bocchi`）**仅 13.9ms** 完成，输出完整的 fib(1000) 大数；
- `fib(35)` 在协程中也即时返回（9227465）。

**⚠️ 陷阱（实测）**：

1. **类型碰撞**：参数哈希用 `%v` 串化，`1` 与 `"1"` 串化结果相同 → 共享缓存条目。实测 `f(x){return x+x;} f(1); f("1");` 输出 `2` `2`（第二个本应输出 `"11"`）。同名函数被不同类型参数复用同一缓存槽。
2. **副作用被跳过**：带副作用的函数（如 io4 的 `formax`/`addlist` 会写 `test.txt`）第二次同参调用**不再执行副作用**，只返回缓存值。
3. **按函数名缓存**：不同作用域/模块中同名函数共享缓存槽（键只有名字，不含定义位置）。

---

## 7. 数据结构

### 7.1 列表

**字面量**：

```bocchi
puts [1, "a", true];
puts [];
puts [1, 2, 3,];          // 支持尾随逗号
```

> ⚠️ 实测：裸列表字面量**不能单独作为语句**（`[1, 2, 3]` 一行会报"无法同步"，解析器把它当作解构赋值左值）。请放进 `puts` / `set` 等表达式中。

**属性 / 方法**：

| 表达式 | 说明 |
|---|---|
| `list.len` | 长度（⚠️ **无括号**） |
| `list.add(x)` | 追加 x |
| `list.remove(x)` | 删除第一次出现的 x |
| `list.removeAt(i)` | 删除下标 i |
| `list[i]` | 下标访问 |
| `list[a:b]` / `list[a:b:step]` | 切片（支持负数索引） |

```bocchi
set l = [1, 2, 3];
set tmp = l.add(4);    // ⚠️ 独立语句 l.add(4); 会解析失败，需放进赋值/表达式
puts l.len;        // 输出：4
puts l[1];         // 输出：2
puts l[0:2];       // 输出：[1, 2]
```

> ⚠️ 实测：以 `变量.方法(...)` 开头的**独立语句**（如 `l.add(4);`、`s.split(" ");`）会解析失败（语句级解析先走"对象成员赋值"分支，而 `add`/`split` 等是关键字 Token）。放进 `set tmp = ...`、`puts ...` 等表达式位置即正常。

### 7.2 字典

```bocchi
set user = {"name": "Alice", "age": 25};
puts user["name"];     // 输出：Alice
user["age"] = 26;
```

> ⚠️ 实测：字典字面量的 key **必须是带引号的字符串**（`{"name": ...}`）；裸标识符 `{name: ...}` 会解析失败（`parseDictLiteral` 要求 String key）。
> ⚠️ **逗号可选**：`{ "a": 1 "b": 2 }` 也能解析（源码 `parseDictLiteral` 中逗号为可选项），但建议还是写逗号。

### 7.3 快速列表生成器

| 函数 | 说明 |
|---|---|
| `range(n)` | `[0, 1, ..., n-1]` |
| `range(a, b)` | `[a, ..., b-1]` |
| `range(a, b, step)` | 指定步长 |
| `repeat(value, count)` | 重复 count 次 |

```bocchi
putsf range(5);          // [0,1,2,3,4]
putsf range(1, 6);       // [1,2,3,4,5]
putsf range(0, 10, 2);   // [0,2,4,6,8]
putsf repeat("hi", 3);   // ["hi", "hi", "hi"]
```

> ⚠️ `zeros()` / `ones()` / `seq()` 不可用（未接入解析器）。

### 7.4 内嵌变量字符串

在 `puts` / `putsf` / `putsr` 的字符串中可使用 `{var}` 插值：

```bocchi
set name = "Bocchi";
puts "Hello, {name}!";    // 输出：Hello, Bocchi!
```

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

### 8.2 继承（`obj 子类 ? 父类`）

```bocchi
obj Dog ? Animal {
    let breed = "未知品种";
    let name = "1";

    fn speak() {
        return "汪汪！我是" + this$name + "，品种是" + this$breed;
    }
    fn wagTail() {
        return this$name + " 摇尾巴";
    }
}

set myDog = Dog;
puts myDog$name;      // 输出：1（子类覆盖了父类的 name）
puts myDog$breed;     // 输出：未知品种
puts myDog$speak();   // 输出：汪汪！我是1，品种是未知品种
puts myDog$getAge();  // 输出：0（方法继承自父类；let 变量不继承）
```

### 8.3 访问成员与方法

| 写法 | 说明 |
|---|---|
| `obj$member` | 属性访问（`$`） |
| `obj$method(args)` | 方法调用 |
| `this$member` / `this$method(args)` | 对象内部访问自身 |
| `super.method(args)` | 调用父类方法(未加入) |

> ⚠️ `obj$member` 访问后**不能再接下标**（如 `req$query["name"]` 会解析失败），需先存入中间变量。

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

### 9.3 重写输出（`putsr`）

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

> ⚠️ 实测：每次 `input()` 都会新建 `bufio.Reader`，**多行管道输入会被第一个 `input()` 吞掉缓冲**，因此带多个 `input()` 的脚本（如 `REPL-cmdrode.bocchi`）无法用一次性的管道输入驱动，只能交互式运行（或按行延时写入 stdin）。交互式控制台下无此问题。

### 9.5 窗口与弹窗

```bocchi
title("Bocchi Script");        // 设置控制台窗口标题
popup("操作完成！");            // 系统弹窗
clear;                          // 清屏（无括号也行）
```

### 9.6 退出

```bocchi
exit;         // 退出码 0
exit(1);      // 指定退出码
exit("2");    // 字符串转数字
```

---

## 10. 内置 API

### 10.1 数学函数

| API | 说明 |
|---|---|
| `sin(x)` / `cos(x)` / `tan(x)` | 三角函数 |
| `sqrt(x)` | 平方根（参数不能为负） |
| `log(x)` / `log10(x)` | 自然对数 / 常用对数 |
| `pow(a, b)` 或 `a ^ b` | 幂 |
| `floor(x)` / `ceil(x)` / `round(x)` | 向下 / 向上 / 四舍五入 |

### 10.2 随机数

| API | 说明 |
|---|---|
| `rand()` | `[0, 1)` 浮点 |
| `rand(n)` | `[0, n)` |
| `rand(a, b)` | `[a, b]`（整型） |

```bocchi
puts rand();
puts rand(1, 10);
```

### 10.3 类型转换

| 写法 | 说明 |
|---|---|
| `expr.int` | 转整数（截断） |
| `expr.float` | 转浮点 |
| `xxx.string` | 转字符串 |

```bocchi
set s = "3.14";
set f = s.float;      // 3.14
set n = (3.9).int;      // 3（⚠️ 数字字面量后直接 .int 会被词法吞成小数，需加括号）
set t = (123).string;   // "123"（同上，变量则无需括号：s.float ✓）
```

### 10.4 字符串方法

| 表达式 | 说明 |
|---|---|
| `s.len` | 长度（**无括号**） |
| `s.split(sep)` | 按 sep 分割为列表 |
| `s.replace(old, new)` | 全部替换 |

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
| `file.scan(path)` | 扫描二维码（⚠️ 不是 `qr.scan`） |
| `dir.delete(path)` | 递归删除目录 |
| `dir.image.show(path)` | 显示图片（⚠️ 不是 `image.show`） |
| `dir.image.addText(path, text, size, x, y, color)` | 图片叠加文字；`color` 为 `"R,G,B"` |

### 10.6 时间

```bocchi
set nowtime = time.now();
puts time.format(nowtime, "2006-01-02 15:04:05");   // ⚠️ now 是关键字，不能作变量名
time.sleep(2);
```

### 10.7 内存命名空间

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

### 10.8 系统命令与参数

| API | 说明 |
|---|---|
| `os.cmd(str)` | 普通权限执行（危险命令被拦截） |
| `os.admin_cmd(str)` | 请求管理员权限执行（Windows） |
| `os.args` | 参数列表 |
| `os.args[i]` | 第 i 个参数 |
| `os.argc` | 参数数量 |

> 🛠️ **中文编码（2026-10-28 已修复）**：`os.cmd` / `os.admin_cmd` 捕获子进程输出时，先探测是否合法 UTF-8——合法（如 `bocchi` 等 Go 程序的输出）直接用，否则（如 `cmd` 内建命令的 GBK 输出）自动按 GBK 解码。实测 `puts os.cmd("bocchi us.bocchi");` 的 `执行用时` 不再乱码，GBK 字节流（`type gbk文件`）也正常。注意：系统里已安装的 `C:\Program Files\Bocchi\bocchi.exe` 仍是旧版（只按 GBK 解码），需要 `bocchi -env install` 或 `bocchi -install` 覆盖更新后才同样修复。

### 10.9 键盘 / 鼠标

**键盘**：`keyboard.press("a")` / `keyboard.release("a")` / `keyboard.type("text")` / `keyboard.record([秒 [, 文件]])` / `keyboard.stopRecord()`

**鼠标**：`mouse.move(x, y)` / `mouse.click()` 或 `mouse.click(left/right/middle)` / `mouse.scroll(delta)` / `mouse.getPos()`（返回 `{x, y}` 字典）

### 10.10 图像识别与截图

> ⚠️ 路径在**括号内**，`region` / `confidence` / `time.sleep(...)` 在**括号外**，空格分隔。

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

---

## 11. 模块与包管理

### 11.1 自动加载（`use` 已弃用）

所有**已安装的包**在每次脚本运行开始时自动加载到同一全局环境（`loadAllPackages`），无需 `use`。本机验证：安装 `io4` 后运行任何脚本都会先执行 `io4.tuka` 的顶层语句。

### 11.2 直接引入脚本（`mod`）

```bocchi
mod "lib/helpers.bocchi";
```

路径限制：必须为相对路径、不含 `..`、扩展名为 `.bocchi` / `.tuka` / 无扩展名。作用：在当前环境中执行目标脚本，等同于"源码包含"（重复引用会被跳过）。

### 11.3 包安装

```bash
bocchi -install package_name          # 安装包
bocchi -install io4.tuka              # 支持 .tuka 扩展包
bocchi -env install                   # 安装解释器（随后检测并询问安装 io4.tuka）
bocchi -list                          # 列出已安装（本机输出：1. io4）
bocchi -uninstall package_name
```

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

### 13.2 `@go`（已弃用）

### 13.3 同步变量到全局（`ok`）

```bocchi
set localVar = 0;
go {
    ok(localVar) = 42;   // 将 42 同步到全局 localVar
}
```

### 13.4 等待所有协程（`wait_all`）

```bocchi
go { ... }
go { ... }
wait_all;    // 阻塞直到所有 go / @go 完成
```

> ⚠️ `go { ... }` 结尾**不要加分号**（见 [§5.6](#56--块语句后面不要写分号42-实测陷阱)）。`go` 可以出现在函数体内部。

---

## 14. 服务器功能（4.2 新增）

`server` 命名空间基于 Go `net/http` 实现，提供最小化 HTTP 服务器。本章所有行为均在本机用 `Bocchi4.2.exe` 实际运行验证。

### 14.1 路由注册

| API | 说明 |
|---|---|
| `server.get(path, handler)` | 注册 GET 路由 |
| `server.post(path, handler)` | 注册 POST 路由 |
| `server.put(path, handler)` | 注册 PUT 路由 |
| `server.delete(path, handler)` | 注册 DELETE 路由 |

- `path` 必须是字符串字面量/表达式结果为字符串；
- `handler` 必须是**函数**（`fn` 命名函数、匿名函数 `fn(params) {...}` 或 `lambda` 均可）；
- 同一 `method + path` 重复注册时**后者覆盖前者**。

```bocchi
server.get("/", fn(req) {
    return "Hello Bocchi!";
});
```

### 14.2 请求对象（`req`）

handler 最多接收一个参数，为 `dict`，含以下键：

| 键 | 类型 | 说明 |
|---|---|---|
| `method` | string | HTTP 方法，如 `"GET"` |
| `path` | string | 请求路径 |
| `query` | dict | URL 查询参数（每个 key 取第一个值） |
| `headers` | dict | 请求头（每个 key 取第一个值） |
| `body` | string | 请求体原文 |

在 handler 内用 `$` 访问：

```bocchi
server.post("/save", fn(req) {
    return "body=" + req$body;
});

server.get("/q", fn(req) {
    set q = req$query;          // ⚠️ 先取出来再下标访问
    set m = req$method;
    return "name=" + q["name"] + " method=" + m;
});
```

> ⚠️ **不能**直接写 `req$query["name"]`（`$` 成员访问后不能再接 `[...]`，会解析失败）。必须先 `set q = req$query;` 再 `q["name"]`。

### 14.3 返回值与响应格式

| handler 返回值 | 响应行为（实测） |
|---|---|
| `string` | `200`，`Content-Type: text/plain; charset=utf-8`，正文为字符串 |
| `dict`（含 `status` / `headers` / `body` 键） | `status` 数字为状态码（默认 200）；`headers` 字典逐项设置响应头；`body` 为正文；未显式设置 Content-Type 时为 text/plain |
| 数字等其它类型 | `200`，`text/plain`，`fmt.Sprint` 结果 |
| 无 `return`（nil） | `200`，空正文 |
| handler 内 panic / 运行出错 | ⚠️ 实测为 **`200`**（不是 500），正文 `服务器内部错误: <错误信息>`（服务器不会崩溃）。原因是内部错误字典里的 `status` 是 Go 整型，而响应写入时只识别浮点状态码，导致状态码丢失、退回 200 |

```bocchi
server.get("/json", fn(req) {
    return {
        "status": 200,
        "headers": {"Content-Type": "application/json"},
        "body": "{\"ok\":true}"
    };
});
```

> `return` 的值会通过 recover 捕获成为响应；handler 内部出错同样被捕获并转为 500。

### 14.4 启动 / 停止 / 等待

| API | 说明 |
|---|---|
| `server.listen(port)` | **阻塞**监听；port 必须为 1–65535 的数字；启动时打印 `[Server] 正在监听 http://localhost:<port>`；启动瞬间**快照**当前已注册路由表 |
| `server.stop()` | 优雅关闭（5 秒超时），打印 `[Server] 已停止`；`listen` 随之返回 |
| `server.wait()` | 阻塞直到服务器停止（每 100ms 轮询） |

最简写法（listen 阻塞主流程）：

```bocchi
server.get("/", fn(req) {
    return "hi";
});

server.get("/stop", fn(req) {
    go {
        time.sleep(0.3);
        server.stop();
    }
    return "stopping...";
});

puts "服务器启动: http://localhost:18360";
server.listen(18360);      // 阻塞直到 stop()
puts "服务器已停止";
```

实测输出：

```
服务器启动: http://localhost:18360
[Server] 正在监听 http://localhost:18360
服务器已停止
[Server] 已停止
```

后台监听写法（listen 放进协程，主流程继续做事）：

```bocchi
server.get("/", fn(req) {
    return "hi";
});
go {
    server.listen(18361);
}
time.sleep(1);                       // 等服务就绪
set r = os.cmd("curl -s http://localhost:18361/");
puts r;                              // 输出：hi
server.stop();
server.wait();                       // 等 listen 协程真正退出
puts "done";
```

### 14.5 未匹配处理（实测）

| 场景 | 行为 |
|---|---|
| 路径未被任何方法注册 | `404 Not Found` |
| 路径已注册但方法不符 | `405 Method Not Allowed`，响应头带 `Allow: GET, PUT`（列出该路径已注册的方法，排序） |

### 14.6 完整示例

```bocchi
server.get("/", fn(req) {
    return "Hello Bocchi!";
});

server.post("/echo", fn(req) {
    return req$body;              // 原样返回请求体
});

server.get("/q", fn(req) {
    set q = req$query;
    return "name=" + q["name"];
});

server.put("/item", fn(req) {
    return "PUT " + req$body;
});

server.delete("/item", fn(req) {
    return "DELETED";
});

server.get("/stop", fn(req) {
    go {
        time.sleep(0.3);
        server.stop();
    }
    return "stopping...";
});

server.listen(18360);
```

### 14.7 ⚠️ 4.2 新增保留字

`server`、`get`、`post`、`put`、`listen`、`stop`、`wait` 在 4.2 中是**全局关键字**（词法层无条件匹配），因此这些词**不能再用作变量名 / 函数名**（4.1 脚本中出现同名标识符会解析失败）。

---

## 15. 测试

### 15.1 定义测试

```bocchi
test "数学运算测试" {
    assert(2 + 2 == 4, "加法测试");
    assert(5 * 5 == 25, "乘法测试");
    assert(10 / 2 == 5, "除法测试");
}

test "字符串测试" {
    set s = "Hello";             // ⚠️ str 是关键字（str() 转换），不能作变量名
    assert(s.len == 5, "长度测试");         // ⚠️ .len 无括号
    assert(s.replace("H", "J") == "Jello", "替换测试");
}
```

### 15.2 运行测试

```bocchi
runTests;
```

- 若脚本中定义了 `test`，脚本执行完毕后会**自动运行**；也可显式 `runTests;`（写在中间的测试立即运行，后面的测试块留到结束合并运行）。
- 失败时 panic（测试模式下），输出带 ✓ / ✗ 与耗时。
- ⚠️ `test "..." { ... }` 结尾**不要加分号**（见 [§5.6](#56--块语句后面不要写分号42-实测陷阱)）。

### 15.3 断言

```bocchi
assert(condition, "错误说明");
assert(condition);   // 消息可选
```

---

## 16. 性能分析

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

## 17. 拆分调试模式

```bash
bocchi "?script.bocchi"
```

输出示例：

```
🔧 拆分执行模式: script.bocchi

✅ 解析成功，共 12 个语句块
────────────────────────────────────────────────────────────
✓ [1/12] 行 3:1 —— 执行成功 (main.TypeDeclaration)
❌ [3/12] 行 15:5 —— 执行失败 (main.FunctionCall)
   错误: Undefined variable: foo
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

## 18. 测试文件参考（bocchi / tuka）

以下文件位于项目目录，可直接 `bocchi <文件>` 运行作为回归测试（⚠️ 带 GUI/键盘/长耗时操作的文件请谨慎运行）：

### 18.1 综合回归测试

| 文件 | 说明 |
|---|---|
| `完整更新测试.bocchi` | 最大综合测试：取整、输出、列表生成器、函数、循环、鼠标、并发 `go/ok`、截图、`test/assert/runTests`、命令行参数、键盘录制、对象/继承、memory 全 API |
| `bocchi4.1p\完整测试用例.bocchi` | 与 `完整更新测试.bocchi` 同源的 4.1 版本用例（内容一致，可对比回归） |

### 18.2 4.2 服务器测试

| 文件 | 说明 |
|---|---|
| `web.tuka` | 🆕 服务器功能测试：`server.get("/")`（返回 `test.html` 内容，实测 200 + text/html）、`server.post("/save")`（请求体写入 `us.bocchi` 并用 `bocchi us.bocchi` 执行，⚠️ 会覆写 `us.bocchi`，勿随便 POST）、`server.get("/run")`（变量 `x` 未定义时实测返回 200 + 正文 `服务器内部错误: Invalid operands for +`）、`server.listen(18362)`。服务器本身启动、路由、404 均正常 |

### 18.3 语言特性测试

| 文件 | 说明 |
|---|---|
| `statictest.tuka` | 切片 `a[2:5]`、io4 的 `del/setlist`、`static int age`、`{name}` 嵌入字符串、`memory` 并发累加、取模 `% / mod`、`os.cmd` 调 `us.bocchi` |
| `newtest.tuka` | 对象继承 + `set Dog$name = "dog"` 成员赋值 |
| `io4.tuka` | io4 扩展库源码（min/max/sum/avg/abs/formax、addlist/del/setlist/isinlist/indexof/countof/join/choice、trim/repeatString/padLeft/padRight、sort/reverse2、isNumber/isString/typeOf、upper/lower、clamp/sign/factorial、shuffle/randString 等）。已安装为包时每次运行脚本自动加载 |
| `bocchi5_yuan.tuka` | 用脚本实现的简易算式预处理：读 `yuan.bocchi`（内容 `12+12`）→ 拆分运算符 → 写入 `yuanout.btmp` → 计算输出 `24` |
| `io3使用说明.markdown` | io3 扩展函数文档（io4 的前一版） |

### 18.4 其它

| 文件 | 说明 |
|---|---|
| `fib数列.bocchi` | `fib_rec(1000)` 朴素递归——得益于 [§6.8 记忆化缓存](#68--函数调用记忆化缓存实测发现)，实测 **13.9ms** 即完成并输出完整大数（无缓存时该脚本会永远跑不完） |
| `REPL-cmdrode.bocchi` | 用脚本实现的迷你 REPL（`cmd.mode` / `putsln` / `exit`） |
| `1.bocchi` | 环境自检：解析 `bocchi -version` 输出判断安装状态，并执行两条 `os.cmd` 维护命令 |
| `us.bocchi` | 最小脚本（输出 `2`），被 `web.tuka` / `statictest.tuka` 通过 `os.cmd` 调用 |
| `yuan.bocchi` | 最小算式文件（`12+12`），被 `bocchi5_yuan.tuka` 读取 |
| `word.bocchi` | 键盘自动化示例（`keyboard.type` + 回车 x10，需 GUI 环境） |
| `test.txt` | io4 的中转文件（`formax` / `addlist` 依赖；顶层加载时会自动创建） |

---

## 19. 常见错误与已知问题

### 19.1 常见错误

| 错误信息 | 原因 | 解决 |
|---|---|---|
| `Undefined variable: xxx` | 变量未声明 | 使用 `set` / `type` / `static` 声明 |
| `Type mismatch` | 类型不匹配 | 检查类型或用 `.int` / `.float` / `.string` 转换 |
| `Invalid operands for +` | 字符串 + 数字 / nil | 使用 `.string` 显式转换 |
| `同步到分号 [行:列]` | 块语句 `}` 后多写了 `;` | 删掉块语句结尾的分号（[§5.6](#56--块语句后面不要写分号42-实测陷阱)） |
| `Syntax error in primary expression: unexpected token ...` | 语法不合法（如 `req$query["x"]`、`obj$m` 后接下标） | 先 `set q = req$query;` 再下标访问 |
| `File write error: open test.txt: Access is denied.` | 当前目录只读（io4 加载时写 `test.txt` 失败） | 在可写目录下运行脚本 |
| `server.listen 端口必须是数字` / `端口必须在 1-65535 之间` | listen 参数非法 | 传入合法数字端口 |
| `server.get 的 handler 必须是函数` | handler 不是函数 | 用 `fn` / `lambda` / 匿名 `fn(...) {...}` |
| `列表索引越界` | 下标超范围 | 检查 `list.len` |
| `非法文件路径` | 路径含 `..` 或绝对路径 | 使用相对安全路径 |
| `图像未找到` | 图像识别失败 | 降低 `confidence`、缩小 `region` |
| `WaitAll` 阻塞 | `go` 未完成 | 检查子协程是否死循环 |

### 19.2 已知问题（4.2 源码实测）

1. **`bocchi -env -record` 不可用**：源码把 `os.Args[2]`（即 `"-record"`）当秒数解析，恒输出 `无效的时长: strconv.ParseFloat: parsing "-record": invalid syntax`。键盘录制请在脚本里用 `keyboard.record(...)`。
2. **版本字符串未更新**：`-version` 仍输出 `(Bocchi 4.1)`、帮助横幅仍为 `1.14.1 (Bocchi 4.1.4)`、REPL 仍为 `v1.14.0 (Bocchi 4.1)`。
3. **`static` 变量赋值无效**：`static int s = 1; s = 2; puts s;` 输出 `1`（赋值写进影子变量，读取永远返回初值），详见 [§3.5](#35-静态变量static)。
4. **函数记忆化缓存碰撞与副作用跳过**：`f(1)` 与 `f("1")` 共享缓存槽；带副作用函数第二次同参调用不执行函数体，详见 [§6.8](#68--函数调用记忆化缓存实测发现)。
5. **`input()` 管道缓冲被吞**：多行一次性管道输入时，第二个及以后的 `input()` 读到 EOF 空串，详见 [§9.4](#94-输入input)。
6. **服务器 handler 崩溃时状态码丢失**：内部错误字典的 `status` 是 Go 整型，响应端只识别浮点 → 崩溃响应实际返回 `200`（而非 500），正文仍为错误信息，详见 [§14.3](#143-返回值与响应格式)。
7. **io4 顶层副作用**：安装 io4 后，每次运行脚本都会创建/覆写 `test.txt`，且 `addlist`/`formax` 依赖该中转文件（并发场景注意竞态）。
8. **`server.listen` 快照路由**：listen 启动后新注册的路由不会被本次监听接受，需先注册全部路由再 listen。
9. **`return fn(...);` 解析失败**：返回命名函数调用必须加括号 `return (fn(...));`，详见 [§6.6](#66-尾调用优化)。
10. **数字字面量后接类型转换失效**：`123.string` / `3.9.int` 会被词法器把 `.` 吞进浮点数而报错，需写 `(123).string` / `(3.9).int`（变量如 `s.float` 不受影响），详见 [§10.3](#103-类型转换)。
11. **`变量.方法(...)` 独立语句失败**：`l.add(4);`、`s.split(" ");` 作为独立语句会解析失败，需放进 `set tmp = ...` 等表达式，详见 [§7.1](#71-列表)。
12. **裸列表字面量不能作语句**：单独一行 `[1, 2, 3]` 报"无法同步"，详见 [§7.1](#71-列表)。
13. **字典字面量必须带引号 key**：`{name: 1}` 报错，需 `{"name": 1}`，详见 [§7.2](#72-字典)。
14. **更多保留字**：`add`（list.add 方法）、`ok`（ok() 同步）、`str`（str() 转换）、`now`（time.now）都是关键字，不能作变量/函数名；`_` 不能作为标识符首字符。
15. **【已修复】os.cmd 中文乱码**：旧版把子进程输出一律按 GBK 解码，导致 `puts os.cmd("bocchi us.bocchi")` 的"执行用时"乱码。本版（2026-10-28 重新编译的 `Bocchi4.2.exe`）已改为**先探测合法 UTF-8、再回退 GBK**（`utf8.Valid` 启发式，三处命令执行路径 `os.cmd` / `os.admin_cmd` / `executeCommand` 同步修复），详见 [§10.8](#108-系统命令与参数)。旧二进制备份在 `Bocchi4.2.exe.bak`；系统已安装的 bocchi 需覆盖更新才生效。

---

## 20. 4.1 → 4.2 变更速查

### 20.1 新增

| 项目 | 说明 |
|---|---|
| 🆕 `server` 命名空间 | `server.get/post/put/delete(path, handler)`、`server.listen(port)`、`server.stop()`、`server.wait()`（详见 [§14](#14-服务器功能42-新增)） |
| 🆕 新 Token | `SERVER` `ROUTE_GET` `ROUTE_POST` `ROUTE_PUT` `LISTEN` `SERVER_STOP` `SERVER_WAIT`（共 7 个，与 4.1.4-2 源码逐一比对确认，无其它 Token 变化） |
| 🆕 新关键字 | `server` `get` `post` `put` `listen` `stop` `wait` |
| 🆕 新增函数（源码对比） | `parseServerExpr` `handleServerRequest` `writeServerResponse` `allowedMethods`（全部为服务器实现） |

### 20.2 破坏性变更

| 项目 | 影响 |
|---|---|
| ⚠️ `get` / `post` / `put` / `listen` / `stop` / `wait` / `server` 成为全局关键字 | 4.1 脚本中同名变量/函数会解析失败，需改名 |

### 20.3 保持不变（与 4.1.4 一致）

- 版本号 `1.14.1`、全部语言特性（变量/运算符/流程控制/函数/对象/io/内置 API）
- `-env` 更新流程（版本检查 + io4 扩展检测，4.1.4 已引入）
- `?<file>` 拆分调试、`-prof` 性能分析、`-link` 打包、`-package` 加密包
- `memory` / `ok()` / `mod` / `test` / `defer` / `try-expect` 等全部沿用

---

## 21. 完整测试报告（2026-10-28 实测）

测试环境：`Bocchi4.2.exe`（本目录）、Windows、已安装包 `io4`（自动加载）。

### 21.1 测试结果总表

| 测试文件 | 结果 | 实测要点 |
|---|---|---|
| `us.bocchi` | ✅ 通过（exit 0） | 输出 `2`，1.1ms |
| `yuan.bocchi` | ✅ 通过 | 内容 `12+12`，被 `bocchi5_yuan.tuka` 读取 |
| `statictest.tuka` | ✅ 通过（exit 0） | 切片 `[3 4 5]`、`del/setlist`、`{name}/{age}` 嵌入、memory 并发 1000、`%`/`mod` 取模、`os.cmd("bocchi us.bocchi")` 全部正确 |
| `newtest.tuka` | ✅ 通过（exit 0） | 继承 + `set Dog$name = "dog"` 生效，输出 `汪汪！我是dog...` |
| `io4.tuka` | ✅ 通过（exit 0） | 顶层加载无报错（含 `test.txt` 初始化） |
| `bocchi5_yuan.tuka` | ✅ 通过（exit 0） | `12+12` → 输出 `24`，13ms |
| `web.tuka` | ✅ 通过（服务器功能正常） | `GET /` → 200 + text/html（test.html）；`GET /run` → 200 + `服务器内部错误: Invalid operands for +`（x 未定义，⚠️ 状态码应为 500 但实测 200，见 [§19.2-6](#192-已知问题42-源码实测)）；未知路径 → 404；服务器正常监听 18362 并优雅终止 |
| `完整更新测试.bocchi` | ✅ 通过（exit 0，3.19s） | 取整/输出/列表/函数/循环/鼠标（`mouse.getPos()` 成功）/并发/截图（成功生成 463KB 新 `screenshot.png`）/键盘录制（5s 无按键→空 `test_keys.txt`）/对象/继承/memory 全部正常；`test` 断言 **2/2 通过，成功率 100%** |
| `bocchi4.1p\完整测试用例.bocchi` | ✅ 通过（exit 0，3.18s） | 与上一文件内容一致（仅末尾换行差异，SHA256 不同、逐行比对 0 差异），结果相同 |
| `fib数列.bocchi` | ✅ 通过（exit 0，13.9ms） | `fib_rec(1000)` 因记忆化缓存秒出完整大数（[§6.8](#68--函数调用记忆化缓存实测发现)） |
| `REPL-cmdrode.bocchi` | ✅ 通过（exit 0） | 逐行延时驱动 stdin：`putsln a b c` → 输出 a b c，`exit` → 正常退出；⚠️ 一次性管道输入会因 `input()` 缓冲失效（[§19.2-5](#192-已知问题42-源码实测)） |
| `1.bocchi` | ⚠️ **未运行** | 内含 `os.cmd("copy /Y BC2027.4.exe 'C:\Program Files\Bocchi\bocchi.exe'")`，会覆盖已安装解释器，测试时已跳过 |
| `word.bocchi` | ⚠️ **未运行** | 会向当前活动窗口注入 10 次 `111`+回车，无人值守环境不安全 |

### 21.2 测试中发现的解释器行为（已写回本文档）

1. **函数调用全局记忆化缓存**：`fib(1000)` 13.9ms；但参数按 `%v` 串化导致 `f(1)`/`f("1")` 缓存碰撞（实测 `f("1")` 返回 `2`），带副作用函数第二次调用跳过函数体 → [§6.8](#68--函数调用记忆化缓存实测发现)。
2. **`static` 变量只读**：`static int s = 1; s = 2; puts s;` → `1`。大测试里 `static int result1 = 1; result1 = fib(35);` 因此输出 `1` 而非 9227465 → [§3.5](#35-静态变量static)。
3. **handler 崩溃响应状态码丢失**（500→200）→ [§14.3](#143-返回值与响应格式)。
4. **`input()` 多行管道输入失效** → [§9.4](#94-输入input)。
5. 旧 README 两处错误实证：`greet("Bob"+"Hi")` 实际输出 `Hello, BobHi!`；`calculate(2,3,4)` 实际输出 `<nil>`（非 9）。

### 21.3 测试产物

- `screenshot.png`（463KB，本次测试新生成；旧截图备份在 `screenshot.old.png`）
- `test_keys.txt`（0 字节：录制 5 秒内无按键，行为正确）
- `yuanout.btmp`（`bocchi5_yuan.tuka` 重新生成；旧文件备份 `yuanout.btmp.bak`）
- `test.txt`（io4 自动加载时的中转文件）

---

## 22. 代码示例逐条测试报告（2026-10-28 实测）

本文档全部 55 个 ```bocchi 示例块与 9 个 ```bash 命令块均已用 `Bocchi4.2.exe` 逐条实测。判定标准：**解释器无报错输出**（⚠️ 解释器出错后仍以退出码 0 结束，需看输出中是否出现 `❌ 错误发生`）。

### 22.1 汇总

- ✅ **直接可运行**：43 个 bocchi 块（含 2 个故意演示错误的"错误写法"块，报错符合预期）
- ❌ **实测失败、已修正文档**：10 个块（修正后已复测通过，见 §22.3）
- 🔀 **需改造才能自动运行**：2 个（`break` 占位符、`go { ... }` 伪代码——已用等价写法验证）
- ⏭️ **跳过自动运行**：`popup`（阻塞弹窗）、`-env install/uninstall`、`-install/-uninstall`、`-package write/install`（系统级/交互式）
- 4 个无语言标记的 ``` 块为**输出示例**，非代码

### 22.2 逐条结果

**✅ 直接通过（43 个 bocchi 块）**：

| 章节 | 示例 | 实测 |
|---|---|---|
| §1.2 / §5.1 / §5.3 / §5.4 / §9.1~9.3 | puts/putsf/putsr、if-else（pre: x=-1）、loop、for-in、for..in、多表达式 `puts x, "..."`、`puts num : 2`（→3.14） | 全部输出与注释一致 |
| §3.2~3.5 | 多变量 `set a,b,c,d,e`、type/static 声明（已改带引号 key） | ✅ |
| §4.1~4.3 | 算术+取模 `%`/`mod`、比较、and/or（pre: x=50） | ✅ 14/1024/1/-1/1.5/2、true、in range |
| §6.2~6.8 | 默认参数 9/1024、可变参数 6/15、fn+go 定义、lambda、rock、记忆化 fib | ✅ |
| §7.3 / §7.4 | range/repeat、`{name}` 嵌入 | ✅ |
| §8.1~8.2 | obj Animal / Dog 继承（Dog 前置 Animal） | ✅ 输出 1/未知品种/汪汪！…/0 |
| §9.6 | `exit;`→0、`exit(1);`→1、`exit("2");`→2 | ✅ 退出码正确 |
| §10.1~10.9 | rand、try/expect、defer（Cleanup/Late/Latest）、go、ok()（→42）、memory（→1000）、mod 引入脚本（输出 yes） | ✅ |
| §10.11 | 图像：`screenshot`→`image_find` 全屏图找到（得分 1.00）、`image_exists` 真/假、region 截图 | ✅ |
| §13 / §14.1~14.6 | 服务器：路由注册片段、完整例自停版（GET /→hi、/stop 优雅退出）、后台 listen+curl、全部路由（GET/POST/PUT/DELETE/404）端到端 | ✅ |
| §15 | test 块自动运行（2/2 通过 100%）、runTests、assert（通过版；顶层失败版只警告不崩溃） | ✅ |
| §5.6 / §19 | 错误写法块：块语句后加 `;` → 实测报 `同步到分号` ✓（符合文档描述） | ✅ |

**✅ bash 命令块（9 个）**：`-help`、`-env`（版本检查→io4 检测→状态）、`-env status`、`-list`（io4）、`bocchi hello.bocchi`、`-prof`（报告生成）、`"?script"` 拆分调试（3 块 2 过 1 败、报 `行:列`）、`-link`（生成 23MB 独立 exe 且可运行）、`-package make`（需密码，生成 `.enc.bocchi`）全部验证；`-env install/uninstall`、`-install/-uninstall`、`-package write/install` 因会改系统/交互式而跳过。

### 22.3 实测失败的示例与修正（10 处）

| # | 原示例 | 失败原因（实测） | 已修正为 |
|---|---|---|---|
| 1 | `set dict1 = {name: "Bob", age: 25};`（§3.1/3.3/3.5/7.2） | 字典 key 必须是字符串字面量，报 `同步到右花括号` | `{"name": "Bob", "age": 25}` ✅ |
| 2 | `type bool ok = true;`（§3.3） | `ok` 是关键字（ok() 同步语句） | `type bool flag = true;` ✅ |
| 3 | `fn add(a, b) {...}`、`set add = lambda...`（§6.1/6.5） | `add` 是关键字（list.add 方法） | 改名 `add2` ✅ |
| 4 | `return fib_iter(n, 0, 1);`（§6.6） | `return fn(...);` 尾调用解析 bug，报 `同步到…` | `return (fib_iter(n, 0, 1));` ✅ 实测 fib(10)=55 |
| 5 | 裸列表字面量一行 `[1, "a", true]`（§7.1） | 被当作解构赋值左值，报 `无法同步` | 放进 `puts [1, "a", true];` ✅ |
| 6 | `l.add(4);` 独立语句（§7.1） | 语句级"对象成员赋值"分支对关键字方法名 panic | `set tmp = l.add(4);` ✅ |
| 7 | `set n = 3.9.int;`、`set t = 123.string;`（§10.3） | 词法器把 `3.9.` / `123.` 吞成浮点，后续 token 报错 | `(3.9).int` / `(123).string` ✅ |
| 8 | `set now = time.now(); time.format(now, ...)`（§10.6） | `now` 是关键字（time.now），表达式里用不了 | 变量改名 `nowtime` ✅ 实测输出 2026-10-28 20:54:16 |
| 9 | `set str = "Hello"; str.len`（§15.1） | `str` 是关键字（str() 转换） | 改名 `s` ✅ |
| 10 | §14.4 后台监听示例无路由，`curl` 得 404 | 块本身能跑但演示效果差 | 补充 `server.get("/", ...)` ✅ 实测输出 hi |

> 另：测试驱动中发现 `_` 不能作为变量名首字符（报 `Unknown character: _`）。

---

祝您使用 Bocchi 4.2 编程愉快！ 🎉


# io4 扩展方法文档
ver1.0 2026.10.28

> 文件来源：`io4.tuka`（595 行）
> 语言：Bocchi 4.2（解释器 v1.14.1）
> 本文档参照《io3使用说明.markdown》的结构撰写，全部函数均在本机用 `Bocchi4.2.exe` 逐条实测（输出见各函数"实测"栏）。
> io4 是 io3 的替代版：已安装为包时（`bocchi -list` 显示 io4），**每次运行任何脚本都会自动加载**全部函数；旧扩展 io2/io3 会在 `bocchi -env` 时提示移除。

---

## 与 io3 的差异（迁移必读）

| 项目 | io3 | io4 |
|---|---|---|
| `repeatstring` / `repeatString` | ✅ 全小写 | ⚠️ **改名 camelCase**，旧名实测 `Undefined variable` |
| `padleft` / `padLeft` | ✅ 全小写 | ⚠️ **改名 camelCase**，旧名不可用 |
| `padright` / `padRight` | ✅ 全小写 | ⚠️ **改名 camelCase**，旧名不可用 |
| 排序 / 反转 | 无 | 🆕 `sort` / `reverse2` |
| 类型判断 | 无 | 🆕 `isNumber/isString/isBool/isList/isDict/typeOf` |
| 首尾元素 | 无 | 🆕 `first` / `last` |
| 字符串套件 | 无 | 🆕 `startsWith/contains2/splitLines/charAt/subString/reverseString/isEmpty/upper/lower` |
| 数学扩展 | 无 | 🆕 `clamp/sign/factorial/deg2rad/rad2deg/lerp` |
| 路径 / 平台 / 随机 | 无 | 🆕 `fileExists/basename/extname/isWindowsOS/getCwd/randString/shuffle/dump` |
| 顶层副作用 | 无 | ⚠️ 加载时执行 `file.write("test.txt","")` 并占用全局变量 `lin/x/i/a/file2` |

> ⚠️ 顶层副作用：io4 每次加载会**创建/覆写 `test.txt`**（`addlist`/`formax` 的中转文件）。在只读目录运行脚本会因写 `test.txt` 失败而报错。

---

## 全局常量

| 名称 | 值 | 用途 |
|---|---|---|
| `void` | `"void"` | 空值哨兵字符串 |
| `nil` | `"<nil>"` | 另一个空值标记（本文件定义但未使用） |

```bocchi
set void = "void";
set nil = "<nil>";
```

---

## 数学类

### `min(...nums)` / `max(...nums)`

求最小/最大值。参数为空返回 `void`。

- **实测**：`min(3,1,4,1,5) → 1`，`max(3,1,4,1,5) → 5` ✅

### `sum(...nums)` / `avg(...nums)`

求和 / 求平均。`avg` 无参数返回 `void`。

- **实测**：`sum(1,2,3) → 6`，`avg(1,3,4324,2342) → 1667.5` ✅

### `abs(x)`

绝对值。**实测**：`abs(-10) → 10` ✅

### `clamp(x, lo, hi)`

限制在 `[lo, hi]`。**实测**：`clamp(10, 0, 5) → 5` ✅

### `sign(x)`

符号：`-1 / 0 / 1`。**实测**：`sign(-3) → -1`，`sign(0) → 0`，`sign(7) → 1` ✅

### `factorial(n)`

阶乘（递归）。**实测**：`factorial(5) → 120` ✅

> ⚠️ 递归函数在 4.2 有全局记忆化缓存（同参秒回），但 `factorial` 无基准条件保护，`factorial(0)` 会无限递归。

### `deg2rad(d)` / `rad2deg(r)`

角度 ↔ 弧度。**实测**：`deg2rad(45) → 0.7853981633974483`，`rad2deg(0.7853981633974483) → 45` ✅

### `lerp(a, b, t)`

线性插值。**实测**：`lerp(1, 2, 0.5) → 1.5` ✅

### `formax(a, b)` ⚠️ 有坑

截取数字字符串转浮点。

- **参数**：`a` 数字字符串，`b` 截取位数
- **实测**：`formax("3.1415926", 3) → 3.141`（**不是 3.14**！实际取前 `b+2` 个字符再转浮点）
- **坑（实测）**：
  1. **不清空 `test.txt`**：连续调用不同参数会叠加损坏——`formax("3.1415926",3)` 后接 `formax("2.71828",3)` 报错 `无法将字符串 '3.1412.718' 转换为浮点数`；
  2. **记忆化副作用跳过**：同参数第二次调用直接返回缓存值（`formax` 函数体不会执行，`test.txt` 不再追加）；
  3. 依赖全局中转文件 `test.txt`。

```bocchi
fn formax(a,b) {
    //file.write("test.txt", "");
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

### `addlist(a, b)`

合并两个列表（lambda，旧版实现，实测可用但脆弱）。

- **实测**：`addlist([1,2],[3,4]) → [1 2 3 4]` ✅
- **说明**：通过 `test.txt` 序列化中转（写 → 读 → **删除**），循环条件 `i < x.len+1` 与外部全局变量 `i/x/a/lin` 耦合；对含空格或特殊字符的元素容易解析错误。
- 当前更安全的选择：`memory.append`。

### `del(list1, index)`

删除指定下标元素（依赖 `addlist`）。**实测**：`del(["a","b","c","d","e"], 2) → [a b d e]` ✅

### `setlist(listc, index, index2)`

替换指定下标元素（依赖 `addlist`）。**实测**：`setlist(["a","b","c"], 1, "X") → [a X c]` ✅

### `isinlist(list1, value)` / `indexof(list1, value)` / `countof(list1, value)`

包含判断 / 首次下标（找不到 `-1`）/ 出现次数。

- **实测**：`isinlist([1,2,3,4,5],3) → true`，`indexof(...) → 2`，`countof([1,2,3,4,5,3],3) → 2` ✅

### `join(list1, sep)`

用分隔符拼字符串。**实测**：`join([1,2,3,4,5], ".") → "1.2.3.4.5"` ✅

### `choice(list1)`

随机取一个元素；空列表返回 `void`。**实测**：`choice([1,2,3,4,5]) → 1`（随机）✅

### `sort(arr, reverse = false)` 🆕

冒泡排序（原地）。

- **参数**：`arr` 列表，`reverse = true` 时降序
- **实测**：`sort([3,1,4,1,5,9,2,6]) → [1 1 2 3 4 5 6 9]`，`sort(..., true) → [9 6 5 4 3 2 1 1]` ✅

### `reverse2(arr)` 🆕

原地反转。**实测**：`reverse2([1,2,3,4]) → [4 3 2 1]` ✅

### `first(arr, d = void)` / `last(arr, d = void)` 🆕

首 / 尾元素；空列表返回默认值 `d`。**实测**：`first([9,8,7]) → 9`，`last([9,8,7]) → 7` ✅

### `shuffle(arr)` 🆕

Fisher–Yates 洗牌（原地）。**实测**：`shuffle([1,2,3,4,5]) → [3 4 5 1 2]`（随机）✅

---

## 字符串类

> ⚠️ io3 的 `repeatstring` / `padleft` / `padright` 在 io4 中**已改名**为 `repeatString` / `padLeft` / `padRight`（camelCase），旧名实测不可用。

| 函数 | 说明 | 实测 |
|---|---|---|
| `trim(s)` | 去空格（`replace(" ","")`，**不处理** `\t`/`\n`/首尾空白） | `trim("  123  ") → "123"` ✅ |
| `repeatString(s, n)` | 重复 n 次 | `repeatString("hello",3) → "hellohellohello"` ✅ |
| `padLeft(s, pad, total)` | 左填充 | `padLeft("5","0",3) → "005"` ✅ |
| `padRight(s, pad, total)` | 右填充 | `padRight("5","0",3) → "500"` ✅ |
| `startsWith(s, prefix)` 🆕 | 前缀判断 | `startsWith("hello","he") → true`，`startsWith("hello","xy") → false` ✅ |
| `contains2(s, sub)` 🆕 | 包含判断（`split(sub).len > 1`） | `contains2("hello","lo") → true` ✅ |
| `splitLines(s)` 🆕 | 按行拆分 | `splitLines("a\nb") → [a b]` ✅ |
| `charAt(s, i)` 🆕 | 第 i 个字符；越界返回 `""` | `charAt("hello",4) → "o"` ✅ |
| `subString(s, a, b)` 🆕 | 截取 `[a, b)` | `subString("hello",1,4) → "ell"` ✅ |
| `reverseString(s)` 🆕 | 反转 | `reverseString("hello") → "olleh"` ✅ |
| `isEmpty(s)` 🆕 | 空串判断 | `isEmpty("") → true`，`isEmpty("h") → false` ✅ |
| `upper(s)` 🆕 | 转大写（仅 ASCII） | `upper("hello") → "HELLO"` ✅ |
| `lower(s)` 🆕 | 转小写（仅 ASCII） | `lower("HELLO") → "hello"` ✅ |

---

## 类型判断类 🆕

| 函数 | 说明 | 实测 |
|---|---|---|
| `isNumber(x)` | 数字（`x + 0` 不报错） | `isNumber(3241) → true`，`isNumber("32w") → false` ✅ |
| `isString(x)` | 字符串（`x.split("")` 不报错） | `isString("x") → true` ✅ |
| `isBool(x)` | 布尔 | `isBool(true) → true`，`isBool(1) → false` ✅ |
| `isList(x)` | 列表（`x.len` 不报错） | `isList([1,2]) → true` ✅ |
| `isDict(x)` | 字典（`x["__io3_probe__"]` 不报错） | `isDict({"a":1}) → true` ✅ |
| `typeOf(x)` | 统一类型名：`bool/num/str/list/dict/unknown` | `typeOf([1]) → list`，`typeOf(1.5) → num`，`typeOf("s") → str`，`typeOf(true) → bool` ✅ |
| `dump(x)` | 调试串：`[类型]:值` | `dump([1,2]) → "[list]:[1 2]"` ✅ |

---

## 文件与路径类

### `fileadd(file1, txt)`

向文件追加文本（lambda）。

- **⚠️ 实测坑**：目标文件**必须已存在**——`fileadd` 先 `file.read`，文件不存在直接报 `File read error`；
- 会覆盖全局变量 `file2`；返回 `""`。
- **实测**：文件含 `"abc"` 时 `fileadd("f.txt","def")` → 文件变为 `"abcdef"` ✅

### `fileExists(p)` 🆕

文件是否存在（`file.read` + try/expect）。**实测**：存在 → `true`，不存在 → `false` ✅

### `basename(p)` / `extname(p)` 🆕

路径取文件名 / 扩展名。

- ⚠️ **只按 `/` 分割**：Windows 反斜杠路径（`C:\a\b.txt`）不会被拆分，`basename` 原样返回整个路径；
- **实测**：`basename("a/b/c.txt") → "c.txt"`，`extname("a/b/c.txt") → ".txt"`，无扩展名返回 `""` ✅

---

## 平台与随机类

| 函数 | 说明 | 实测 |
|---|---|---|
| `isWindowsOS()` 🆕 | 是否 Windows（`echo %OS%` 展开判断） | 本机 `true` ✅ |
| `getCwd()` 🆕 | 当前目录（Windows 用 `cd`，Unix 用 `pwd` 并去尾换行） | `C:\CODE\GO\boc23\examples_check` ✅ |
| `randString(n)` 🆕 | 随机字符串（小写字母+数字） | `randString(6) → "tugb4f"`（随机）✅ |

---

## 工具类

### `isVoid(x)`

判断是否为空值（与全局常量 `void` 比较）。

- **实测**：`isVoid(void) → true`，`isVoid(1) → false` ✅

---

## 分类速查表

| 类别 | 方法 |
|---|---|
| **数值** | `min` `max` `sum` `avg` `abs` `formax` `clamp` `sign` `factorial` `deg2rad` `rad2deg` `lerp` |
| **列表** | `addlist` `del` `setlist` `isinlist` `indexof` `countof` `join` `choice` `sort` `reverse2` `first` `last` `shuffle` |
| **字符串** | `trim` `repeatString` `padLeft` `padRight` `startsWith` `contains2` `splitLines` `charAt` `subString` `reverseString` `isEmpty` `upper` `lower` |
| **类型判断** | `isNumber` `isString` `isBool` `isList` `isDict` `typeOf` `dump` |
| **文件 / 路径** | `fileadd` `fileExists` `basename` `extname` |
| **平台 / 随机** | `isWindowsOS` `getCwd` `randString` |
| **工具** | `isVoid` |

---

## 依赖关系

```
addlist ──► del
         └─► setlist
addlist / formax ──► test.txt（全局中转文件）
fileadd ──► file2（全局变量，被覆盖）
addlist ──► i / x / a / lin（顶层全局变量，被反复重赋值）
```

### 潜在问题（实测）

1. **`formax` 不清空 `test.txt`**：连续调用不同参数 → 内容叠加 → `无法将字符串 '3.1412.718' 转换为浮点数`；且同参第二次调用因解释器**记忆化缓存**跳过函数体（副作用不执行，返回缓存值）。
2. **`addlist` 实现脆弱**：序列化中转 + 循环与外部变量耦合；执行后 `test.txt` 被删除。`del` / `setlist` 的正确性完全取决于 `addlist`。
3. **`fileadd` 要求文件已存在**：先 `file.read` 后追加，目标缺失直接报错。
4. **`basename` / `extname` 只按 `/` 分割**：Windows 反斜杠路径处理不了。
5. **`trim` 是简化版**：只删空格，不处理 `\t`、`\n` 和首尾空白。
6. **顶层占用全局名**：`lin`、`x`、`i`、`a`、`file2` 在加载时被 io4 占用/初始化，用户脚本同名变量会被覆盖。
7. **`factorial` 无基准保护**：`factorial(0)` 无限递归。
8. **`upper` / `lower` 仅 ASCII**。

---

## 使用示例（实测输出）

```bocchi
// 数学
puts min(3, 1, 4, 1, 5);       // 1
puts max(3, 1, 4, 1, 5);       // 5
puts sum(1, 2, 3);              // 6
puts avg(1, 3, 4324, 2342);    // 1667.5
puts abs(-10);                  // 10
puts clamp(10, 0, 5);           // 5
puts factorial(5);              // 120
puts formax("3.1415926", 3);   // 3.141（注意：非 3.14）

// 列表
puts addlist([1,2],[3,4]);              // [1 2 3 4]
puts del(["a","b","c","d","e"], 2);     // [a b d e]
puts setlist(["a","b","c"], 1, "X");    // [a X c]
puts isinlist([1,2,3,4,5], 3);          // true
puts indexof([1,2,3,4,5], 3);           // 2
puts countof([1,2,3,4,5,3], 3);         // 2
puts join([1,2,3,4,5], ".");            // "1.2.3.4.5"
puts sort([3,1,4,1,5,9,2,6]);           // [1 1 2 3 4 5 6 9]
puts reverse2([1,2,3,4]);               // [4 3 2 1]
puts first([9,8,7]);                    // 9
puts choice([1,2,3,4,5]);               // 随机

// 字符串（注意 camelCase 新名）
puts trim("  123  ");                   // "123"
puts repeatString("hello", 3);          // "hellohellohello"
puts padLeft("5", "0", 3);              // "005"
puts padRight("5", "0", 3);             // "500"
puts startsWith("hello", "he");         // true
puts charAt("hello", 4);                // "o"
puts subString("hello", 1, 4);          // "ell"
puts reverseString("hello");            // "olleh"
puts upper("hello");                    // "HELLO"

// 类型判断
puts typeOf([1]);                       // list
puts dump([1,2]);                       // "[list]:[1 2]"

// 文件（⚠️ 目标必须先存在）
file.write("demo.txt", "abc");
puts fileadd("demo.txt", "def");        // ""（返回值），文件变为 "abcdef"
puts fileExists("demo.txt");            // true

// 平台
puts isWindowsOS();                     // true（Windows）
puts getCwd();                          // 当前目录
puts isVoid(void);                      // true
```

祝您使用 io4 愉快！ 🎉



