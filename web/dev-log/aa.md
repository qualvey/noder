## nn

```
前端，加入选择模板功能。模板文件通过assets/templates/文件夹传递，节点进行tag去重之后，放入
{
            "tag": "Proxy",
            "type": "selector",
            "interrupt_exist_connections": true,
            "outbounds": [
                "urltest"
            ]
        },
        {
            "tag": "urltest",
            "type": "urltest",
            "outbounds": []
        } 
两个outbounds数组后面
```