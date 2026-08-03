<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet
    version="1.0"
    xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    
    <xsl:output method="xml" encoding="UTF-8" indent="no" omit-xml-declaration="yes"/>
    <xsl:strip-space elements="*"/>

    <!-- Root -->
    <xsl:template match="/document">
        <html>
            <head>
                <title><xsl:value-of select="meta/title/@value"/></title>
            </head>
            <body>
                <xsl:apply-templates select="body/*"/>
            </body>
        </html>
    </xsl:template>
    
    <!-- TEXT -->
    <xsl:template match="text">
        <p>
            <xsl:value-of select="."/>
        </p>
    </xsl:template>
    
    <!-- LINK GROUP -->
    <xsl:template match="link-group">
        <p>
            <xsl:apply-templates select="link"/>
        </p>
    </xsl:template>

    <xsl:template match="link">
        <a href="{@href}"><xsl:value-of select="."/></a><br/>
    </xsl:template>
    
    <!-- BOLD -->
    <xsl:template match="bold">
        <p><strong><xsl:value-of select="."/></strong></p>
    </xsl:template>
    
    <!-- CODE -->
    <xsl:template match="code">
        <xsl:choose>
            
            <!-- Code contains a table -->
            <xsl:when test="table">
                <xsl:apply-templates select="table"/>
            </xsl:when>
            
            <!-- Plain code -->
            <xsl:otherwise>
                <pre><code><xsl:value-of select="."/></code></pre>
            </xsl:otherwise>
            
        </xsl:choose>
    </xsl:template>
    
    <xsl:template match="table">
        <table>
            <xsl:apply-templates/>
        </table>
    </xsl:template>
    
    <xsl:template match="tr">
        <tr><xsl:apply-templates/></tr>
    </xsl:template>
    
    <xsl:template match="td">
        <td>
            <xsl:if test="@style">
                <xsl:attribute name="style">
                    <xsl:value-of select="@style"/>
                </xsl:attribute>
            </xsl:if>
            <xsl:value-of select="."/>
        </td>
    </xsl:template>
    
    <!-- ITEM GROUP -->
    <xsl:template match="item-group">
        <ul>
            <xsl:apply-templates select="item"/>
        </ul>
    </xsl:template>
    
    <xsl:template match="item">
        <li>
            <xsl:value-of select="."/>
        </li>
    </xsl:template>
    
</xsl:stylesheet>
