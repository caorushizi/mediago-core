import { config, devConfig } from "./config";
import { getExeExt, mkdir, runCommand } from "./utils";
import { join } from "path";

/**
 * 启动开发服务器
 */
export async function dev() {
  console.log("🚀 启动开发服务器...");
  const command = [
    `go run ${config.CMD_PATH}`,
    `-config='${JSON.stringify(devConfig)}'`,
  ].join(" ");
  await runCommand(command);
}

/**
 * 编译当前平台的开发版本
 */
export async function devBuild() {
  console.log("🔨 编译开发版本...");
  mkdir(config.BIN_DIR);
  const output = join(config.BIN_DIR, config.APP_NAME + getExeExt());
  await runCommand(
    `go build -ldflags "${config.GO_LDFLAGS}" -o ${output} ${config.CMD_PATH}`,
    "编译当前平台二进制文件",
  );
  console.log(`✅ 开发版本编译成功 -> ${output}`);
}
