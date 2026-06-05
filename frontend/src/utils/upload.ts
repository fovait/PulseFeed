import SparkMD5 from "spark-md5";
import { pulsefeedApi } from "../api/pulsefeed";

export const DEFAULT_CHUNK_SIZE = 5 * 1024 * 1024;

export type UploadProgress = {
  phase: "hashing" | "uploading" | "complete";
  completed: number;
  total: number;
};

function readChunk(chunk: Blob) {
  return new Promise<ArrayBuffer>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as ArrayBuffer);
    reader.onerror = () => reject(reader.error || new Error("failed to read file chunk"));
    reader.readAsArrayBuffer(chunk);
  });
}

async function md5Blob(blob: Blob) {
  const buffer = await readChunk(blob);
  return SparkMD5.ArrayBuffer.hash(buffer);
}

async function md5File(file: File, chunkSize = DEFAULT_CHUNK_SIZE, onProgress?: (progress: UploadProgress) => void) {
  const total = Math.ceil(file.size / chunkSize);
  const spark = new SparkMD5.ArrayBuffer();
  for (let index = 0; index < total; index += 1) {
    const start = index * chunkSize;
    const end = Math.min(file.size, start + chunkSize);
    spark.append(await readChunk(file.slice(start, end)));
    onProgress?.({ phase: "hashing", completed: index + 1, total });
  }
  return spark.end();
}

export async function uploadVideoInChunks(file: File, onProgress?: (progress: UploadProgress) => void) {
  const chunkSize = DEFAULT_CHUNK_SIZE;
  const totalChunks = Math.ceil(file.size / chunkSize);
  const fileHash = await md5File(file, chunkSize, onProgress);
  const init = await pulsefeedApi.initChunkUpload({
    filename: file.name,
    file_size: file.size,
    chunk_size: chunkSize,
    total_chunks: totalChunks,
    file_hash: fileHash,
  });
  const uploaded = new Set(init.upload_chunks || []);

  for (let index = 0; index < totalChunks; index += 1) {
    if (uploaded.has(index)) {
      onProgress?.({ phase: "uploading", completed: uploaded.size, total: totalChunks });
      continue;
    }
    const start = index * chunkSize;
    const end = Math.min(file.size, start + chunkSize);
    const chunk = file.slice(start, end);
    const formData = new FormData();
    formData.append("upload_id", init.upload_id);
    formData.append("chunk_index", String(index));
    formData.append("chunk_hash", await md5Blob(chunk));
    formData.append("file", chunk, `${file.name}.part${index}`);
    await pulsefeedApi.uploadChunk(formData);
    uploaded.add(index);
    onProgress?.({ phase: "uploading", completed: uploaded.size, total: totalChunks });
  }

  const complete = await pulsefeedApi.completeChunkUpload(init.upload_id);
  onProgress?.({ phase: "complete", completed: totalChunks, total: totalChunks });
  return complete.play_url || complete.url;
}
